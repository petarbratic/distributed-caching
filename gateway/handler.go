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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var totalUserRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_http_requests_total",
		Help: "Total number of HTTP requests gateway received.",
	},
)

var totalBackendRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_backend_requests_total",
		Help: "Total number of HTTP requests sent to backend.",
	},
)

var totalL1Hits = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_l1_hits_total",
		Help: "Total number of L1 hits.",
	},
)

var totalL2Hits = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_l2_hits_total",
		Help: "Total number of L2 hits.",
	},
)

var totalCacheMisses = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_cache_miss_total",
		Help: "Total number of cache misses.",
	},
)

var requestDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name: "gateway_request_duration_seconds",
		Help: "Total duration of processing a user request (in seconds).",
		Buckets: []float64{
			0.001,
			0.005,
			0.01,
			0.025,
			0.05,
			0.1,
			0.25,
			0.5,
			1,
			2,
			5,
		},
	},
)

var backendDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name: "gateway_backend_duration_seconds",
		Help: "Total duration of backend processing a user request (in seconds).",
		Buckets: []float64{
			0.001,
			0.005,
			0.01,
			0.025,
			0.05,
			0.1,
			0.25,
			0.5,
			1,
			2,
			5,
		},
	},
)

type CacheConfig struct {
	L1MaxEntries int `json:"l1MaxEntries"`
	L2MaxEntries int `json:"l2MaxEntries"`
}

type KeyValue struct {
	Value      []byte
	Expiration time.Time
}

type CacheEntry struct {
	Key   string
	Value KeyValue
}

type Handler struct {
	proxy *httputil.ReverseProxy

	cache      map[string]*list.Element
	cacheOrder *list.List
	mu         sync.Mutex

	redis *redis.Client

	cacheConfig CacheConfig
}

type ResponseWriter struct {
	http.ResponseWriter
	body []byte
}

func (rw *ResponseWriter) Write(data []byte) (int, error) {
	rw.body = append(rw.body, data...)
	return rw.ResponseWriter.Write(data)
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
			L1MaxEntries: 100,
			L2MaxEntries: 1000,
		},
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	totalUserRequests.Inc()

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()

	ttlL1 := 2 * time.Second
	ttlL2 := 4 * time.Second

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
	value, err := h.redis.Get(ctx, key).Bytes()

	if err == nil {
		h.setL1(key, KeyValue{
			Value:      value,
			Expiration: time.Now().Add(ttlL1),
		})

		_, _ = w.Write(value)
		log.Println("L2 HIT:", key)
		totalL2Hits.Inc()
		return
	}

	if err != redis.Nil {
		log.Println("Redis GET error:", err)
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

	log.Println(
		"Backend call duration:",
		time.Since(backendStart),
	)

	// L2 write
	if err := h.redis.Set(ctx, key, rw.body, ttlL2).Err(); err != nil {
		log.Println("Redis SET error:", err)
	}

	// L1 write
	bodyCopy := append([]byte(nil), rw.body...)

	h.setL1(key, KeyValue{
		Value:      bodyCopy,
		Expiration: time.Now().Add(ttlL1),
	})

	log.Println("CACHE MISS:", key)
	totalCacheMisses.Inc()
}

func (h *Handler) getL1(key string) (KeyValue, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	element, found := h.cache[key]
	if !found {
		return KeyValue{}, false
	}

	entry := element.Value.(*CacheEntry)

	if time.Now().After(entry.Value.Expiration) {
		delete(h.cache, key)
		h.cacheOrder.Remove(element)

		log.Println("L1 Expired: ", key)

		return KeyValue{}, false
	}
	h.cacheOrder.MoveToFront(element)

	return entry.Value, true
}

func (h *Handler) setL1(key string, value KeyValue) {
	h.mu.Lock()
	defer h.mu.Unlock()

	maxEntries := h.cacheConfig.L1MaxEntries

	if maxEntries <= 0 {
		return
	}

	if element, found := h.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		entry.Value = value

		h.cacheOrder.MoveToFront(element)
		return
	}

	if len(h.cache) >= maxEntries {
		h.removeLRU()
	}

	entry := &CacheEntry{
		Key:   key,
		Value: value,
	}

	element := h.cacheOrder.PushFront(entry)
	h.cache[key] = element

}

func (h *Handler) removeLRU() {
	element := h.cacheOrder.Back()
	if element == nil {
		return
	}

	entry := element.Value.(*CacheEntry)

	delete(h.cache, entry.Key)
	h.cacheOrder.Remove(element)

	log.Println("L1 EVICTED: ", entry.Key)
}
