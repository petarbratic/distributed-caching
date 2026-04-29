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

type Handler struct {
	proxy *httputil.ReverseProxy
	cache map[string][]byte
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
		cache: make(map[string][]byte),
		redis: rdb,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	start := time.Now()

	ctx := r.Context()
	key := r.URL.RequestURI()

	h.mu.RLock()
	if data, ok := h.cache[key]; ok {
		h.mu.RUnlock()
		w.Write(data)
		log.Println("L1 HIT, total time: ", time.Since(start))
		return
	}
	h.mu.RUnlock()

	val, err := h.redis.Get(ctx, key).Bytes()
	if err == nil {
		h.mu.Lock()
		h.cache[key] = val
		h.mu.Unlock()
		w.Write(val)
		log.Println("L2 HIT, total time: ", time.Since(start))
		return
	}

	rw := &ResponseWriter{
		ResponseWriter: w,
	}

	log.Println("Backend call, total time: ", time.Since(start))
	h.proxy.ServeHTTP(rw, r)

	if err := h.redis.Set(ctx, key, rw.body, 0).Err(); err != nil {
		log.Println("Redis SET error:", err)
	}

	h.mu.Lock()
	h.cache[key] = rw.body
	h.mu.Unlock()

}
