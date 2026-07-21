package main

import (
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

var totalRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_http_requests_total",
		Help: "Total number of HTTP requests gateway received.",
	},
)

type KeyValue struct {
	Value      []byte
	Expiration time.Time
}

type Handler struct {
	proxy *httputil.ReverseProxy
	cache map[string]KeyValue
	mu    sync.RWMutex
	redis *redis.Client
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
		proxy: proxy,
		cache: make(map[string]KeyValue),
		redis: rdb,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	totalRequests.Inc()

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()

	ttlL1 := time.Second
	ttlL2 := 2 * time.Second

	defer func() {
		log.Printf(
			"Request total time: %v, key: %s",
			time.Since(start),
			key,
		)
	}()

	// L1 cache
	h.mu.RLock()
	cached, found := h.cache[key]
	h.mu.RUnlock()

	if found {
		if time.Now().Before(cached.Expiration) {
			_, _ = w.Write(cached.Value)
			log.Println("L1 HIT:", key)
			return
		}

		h.mu.Lock()
		delete(h.cache, key)
		h.mu.Unlock()

		log.Println("L1 EXPIRED:", key)
	}

	// L2 Redis
	value, err := h.redis.Get(ctx, key).Bytes()

	if err == nil {
		h.mu.Lock()
		h.cache[key] = KeyValue{
			Value:      value,
			Expiration: time.Now().Add(ttlL1),
		}
		h.mu.Unlock()

		_, _ = w.Write(value)
		log.Println("L2 HIT:", key)
		return
	}

	if err != redis.Nil {
		log.Println("Redis GET error:", err)
	}

	// Backend
	rw := &ResponseWriter{
		ResponseWriter: w,
	}

	backendStart := time.Now()

	h.proxy.ServeHTTP(rw, r)

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

	h.mu.Lock()
	h.cache[key] = KeyValue{
		Value:      bodyCopy,
		Expiration: time.Now().Add(ttlL1),
	}
	h.mu.Unlock()

	log.Println("CACHE MISS:", key)
}
