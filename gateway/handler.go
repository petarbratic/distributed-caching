package main

import (
	"container/list"
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

	cacheConfig CacheConfig

	backendURL string
	httpClient *http.Client

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

	return &Handler{
		proxy:      proxy,
		cache:      make(map[string]*list.Element),
		cacheOrder: list.New(),
		redis:      rdb,
		cacheConfig: CacheConfig{
			L1MaxEntries:        35,
			L2MaxEntries:        70,
			L1TTLSeconds:        15,
			L2TTLSeconds:        30,
			SemaphoreSize:       15,
			ConcurrentDelayMs:   5,
			BaseLatencyMs:       200,
			SingleFlightEnabled: false,
			BackendTimeoutMs:    7000,
		},
		backendURL: strings.TrimRight(target, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		inFlight: make(map[string]*inFlightCall),
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	totalUserRequests.Inc()

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()

	ttlL1 := time.Duration(h.cacheConfig.L1TTLSeconds) * time.Second
	ttlL2 := time.Duration(h.cacheConfig.L2TTLSeconds) * time.Second

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
		h.setL1(key, KeyValue{
			Value:      value,
			Expiration: time.Now().Add(ttlL1),
		})

		_, _ = w.Write(value)

		//log.Println("L2 HIT: ", key)
		totalL2Hits.Inc()

		return
	}

	//log.Println("CACHE MISS:", key)
	totalCacheMisses.Inc()

	// Backend
	if h.cacheConfig.SingleFlightEnabled {
		h.handleBackendSingleFlight(w, r, key, ttlL1, ttlL2)
		return
	}

	h.handleBackendNormally(w, r, key, ttlL1, ttlL2)

}

func (h *Handler) handleBackendNormally(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	ttlL1 time.Duration,
	ttlL2 time.Duration,
) {
	body, statusCode, err := h.fetchFromBackend(r)

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

	ctx := r.Context()

	h.setL2(ctx, key, body, ttlL2)

	h.setL1(key, KeyValue{
		Value:      append([]byte(nil), body...),
		Expiration: time.Now().Add(ttlL1),
	})
}
