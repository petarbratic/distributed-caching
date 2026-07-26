package main

import (
	"container/list"
	"context"
	"net/http"

	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const l2LRUKey = "gateway:l2:lru"

type Handler struct {
	proxy *httputil.ReverseProxy

	cache      map[string]*list.Element
	cacheOrder *list.List
	mu         sync.Mutex

	redis *redis.Client

	configStore   *GatewayConfigStore
	configMu      sync.RWMutex
	gatewayConfig GatewayConfig
	configVersion int64

	inFlightMu sync.Mutex
	inFlight   map[string]*inFlightCall
}

func NewHandler(target string) (*Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if holder, ok := r.Context().Value(proxyErrorContextKey{}).(*proxyErrorHolder); ok {
			holder.err = err
		}

		http.Error(w, "Backend unavailable", http.StatusBadGateway)
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Path = strings.TrimPrefix(
			req.URL.Path,
			"/api/backend",
		)

		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.TrimPrefix(
				req.URL.RawPath,
				"/api/backend",
			)
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	configStore := &GatewayConfigStore{
		redis: rdb,
	}

	defaultConfig := GatewayConfig{
		L1MaxEntries:           35,
		L2MaxEntries:           70,
		L1TTLSeconds:           15,
		L2TTLSeconds:           30,
		SingleFlightEnabled:    false,
		DistributedLockEnabled: false,
		BackendTimeoutMs:       7000,
	}

	ctx := context.Background()

	if err := configStore.Initialize(ctx, defaultConfig); err != nil {
		return nil, err
	}

	gatewayConfig, configVersion, err := configStore.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &Handler{
		proxy:         proxy,
		cache:         make(map[string]*list.Element),
		cacheOrder:    list.New(),
		redis:         rdb,
		configStore:   configStore,
		gatewayConfig: gatewayConfig,
		configVersion: configVersion,
		inFlight:      make(map[string]*inFlightCall),
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	config, err := h.currentGatewayConfig(r.Context())
	if err != nil {
		http.Error(
			w,
			"Gateway configuration unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}

	totalUserRequests.Inc()

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()

	ttlL1 := time.Duration(config.L1TTLSeconds) * time.Second
	ttlL2 := time.Duration(config.L2TTLSeconds) * time.Second

	defer func() {
		duration := time.Since(start)

		requestDuration.Observe(duration.Seconds())

		//log.Printf("Request total time: %v, key: %s", duration,	key)
	}()

	// L1 cache
	if cached, found := h.getL1(key); found {
		_, _ = w.Write(cached.Value)

		//log.Println("L1 HIT: ", key)
		totalL1Hits.Inc()

		return
	}

	// L2 Redis
	value, found := h.getL2(ctx, key)

	if found {
		h.setL1(
			key,
			KeyValue{
				Value:      value,
				Expiration: time.Now().Add(ttlL1),
			},
			config.L1MaxEntries,
		)

		_, _ = w.Write(value)

		//log.Println("L2 HIT: ", key)
		totalL2Hits.Inc()

		return
	}

	//log.Println("CACHE MISS:", key)
	totalCacheMisses.Inc()

	// Backend
	if config.SingleFlightEnabled {
		h.handleBackendSingleFlight(w, r, key, ttlL1, ttlL2, config)
		return
	}

	h.handleBackendNormally(w, r, key, ttlL1, ttlL2, config)

}

func (h *Handler) handleBackendNormally(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	ttlL1 time.Duration,
	ttlL2 time.Duration,
	config GatewayConfig,
) {
	var (
		body       []byte
		statusCode int
		err        error
	)
	if config.DistributedLockEnabled {
		body, statusCode, err = h.fetchWithDistributedLock(r, key, ttlL2, config)
	} else {
		body, statusCode, err = h.fetchFromBackend(r, config.BackendTimeoutMs)
	}

	if err != nil {
		http.Error(
			w,
			"Backend request failed",
			statusCode,
		)
		return
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(body)

	if statusCode < 200 || statusCode >= 300 {
		return
	}

	if !config.DistributedLockEnabled {
		h.setL2(r.Context(), key, body, ttlL2, config.L2MaxEntries)
	}

	h.setL1(
		key,
		KeyValue{
			Value:      append([]byte(nil), body...),
			Expiration: time.Now().Add(ttlL1),
		},
		config.L1MaxEntries,
	)
}

func (h *Handler) currentGatewayConfig(
	ctx context.Context,
) (GatewayConfig, error) {
	config, version, err := h.configStore.Get(ctx)
	if err != nil {
		return GatewayConfig{}, err
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	if version != h.configVersion {
		h.clearL1()

		h.gatewayConfig = config
		h.configVersion = version
	}

	return h.gatewayConfig, nil
}
