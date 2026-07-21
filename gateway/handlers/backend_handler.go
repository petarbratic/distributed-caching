package handlers

import (
	//"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type KeyValue struct {
	Value      string
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

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return rw.ResponseWriter.Write(b)
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
		// Strip /api prefix before forwarding to backend
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api/backend")
		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/api/backend")
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	return &Handler{proxy: proxy,
		cache: make(map[string]KeyValue),
		redis: rdb,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()
	ttlL1 := time.Second
	ttlL2 := 2 * time.Second

	defer func() {
		log.Printf("Request total time: %v, for key: %v", time.Since(start), key)
	}()

	// L1 Cache
	h.mu.RLock()
	cached, ok := h.cache[key]
	h.mu.RUnlock()

	if ok {
		if time.Now().Before(cached.Expiration) {
			_, _ = w.Write([]byte(cached.Value))
			log.Println("L1 HIT: ", key)
			return
		}

		h.mu.Lock()
		delete(h.cache, key)
		h.mu.Unlock()

		log.Println("L1 Expired: ", key)
	}

	// L2 Redis
	val, err := h.redis.Get(ctx, key).Bytes()
	if err == nil {
		// Write to L1
		h.mu.Lock()
		h.cache[key] = KeyValue{
			Value:      string(val),
			Expiration: time.Now().Add(ttlL1),
		}
		h.mu.Unlock()

		_, _ = w.Write(val)
		log.Println("L2 HIT: ", key)
		return
	}

	if err != redis.Nil {
		log.Println("Redis expired for key: ", key)
	}

	// Backend
	rw := &ResponseWriter{
		ResponseWriter: w,
	}

	backendStart := time.Now()
	h.proxy.ServeHTTP(rw, r)
	log.Println("Backend call duration: ", time.Since(backendStart))

	// Write to L2
	if err := h.redis.Set(ctx, key, rw.body, ttlL2).Err(); err != nil {
		log.Println("Redis SET error:", err)
	}

	// Write to L1
	h.mu.Lock()
	h.cache[key] = KeyValue{
		Value:      string(rw.body),
		Expiration: time.Now().Add(ttlL1),
	}
	h.mu.Unlock()

	log.Println("CACHE MISS")
}
