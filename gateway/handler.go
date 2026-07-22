package main

import (
	"container/list"
	"log"
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
}

func NewHandler(target string) (*Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

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
			L1MaxEntries:  3,
			L2MaxEntries:  10,
			L1TTLSeconds:  2,
			L2TTLSeconds:  4,
			SemaphoreSize: 3,
		},
		backendURL: strings.TrimRight(target, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
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

		log.Printf(
			"Request total time: %v, key: %s",
			duration,
			key,
		)
	}()

	// L1 cache
	if cached, found := h.getL1(key); found {
		_, _ = w.Write(cached.Value)

		log.Println("L1 HIT: ", key)
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

		log.Println("L2 HIT: ", key)
		totalL2Hits.Inc()

		return
	}

	// Backend

	totalBackendRequests.Inc()

	rw := &ResponseWriter{
		ResponseWriter: w,
	}

	backendStart := time.Now()

	h.proxy.ServeHTTP(rw, r)

	duration := time.Since(backendStart)
	backendDuration.Observe(duration.Seconds())

	log.Printf(
		"Backend call duration: %v, for key: %v",
		time.Since(backendStart),
		key,
	)

	// L2 write
	h.setL2(ctx, key, rw.body, ttlL2)

	// L1 write
	bodyCopy := append([]byte(nil), rw.body...)

	h.setL1(key, KeyValue{
		Value:      bodyCopy,
		Expiration: time.Now().Add(ttlL1),
	})

	log.Println("CACHE MISS:", key)
	totalCacheMisses.Inc()
}
