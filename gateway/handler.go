package main

import (
	"container/list"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
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

type inFlightCall struct {
	done       chan struct{}
	data       []byte
	statusCode int
	err        error
}

type proxyErrorHolder struct {
	err error
}

type proxyErrorContextKey struct{}

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
			L1MaxEntries:        3,
			L2MaxEntries:        10,
			L1TTLSeconds:        2,
			L2TTLSeconds:        4,
			SemaphoreSize:       3,
			ConcurrentDelayMs:   5,
			BaseLatencyMs:       500,
			SingleFlightEnabled: false,
			BackendTimeoutMs:    5000,
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

	log.Println("CACHE MISS:", key)
	totalCacheMisses.Inc()

	// Backend
	if h.cacheConfig.SingleFlightEnabled {
		h.handleBackendSingleFlight(w, r, key, ttlL1, ttlL2)
		return
	}

	h.handleBackendNormally(w, r, key, ttlL1, ttlL2)

}

func (h *Handler) getOrCreateInFlight(key string) (*inFlightCall, bool) {
	h.inFlightMu.Lock()
	defer h.inFlightMu.Unlock()

	if call, exists := h.inFlight[key]; exists {
		return call, false
	}

	call := &inFlightCall{
		done: make(chan struct{}),
	}

	h.inFlight[key] = call

	return call, true
}

func (h *Handler) finishInFlight(
	key string,
	call *inFlightCall,
	data []byte,
	statusCode int,
	err error,
) {
	h.inFlightMu.Lock()
	defer h.inFlightMu.Unlock()

	call.data = append([]byte(nil), data...)
	call.statusCode = statusCode
	call.err = err

	delete(h.inFlight, key)
	close(call.done)
}

func (h *Handler) fetchFromBackend(r *http.Request) ([]byte, int, error) {
	totalBackendRequests.Inc()

	backendStart := time.Now()

	timeout := time.Duration(h.cacheConfig.BackendTimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	errorHolder := &proxyErrorHolder{}

	ctx = context.WithValue(
		ctx,
		proxyErrorContextKey{},
		errorHolder,
	)

	requestCopy := r.Clone(ctx)
	recorder := httptest.NewRecorder()

	h.proxy.ServeHTTP(recorder, requestCopy)

	duration := time.Since(backendStart)
	backendDuration.Observe(duration.Seconds())

	log.Printf(
		"Backend call duration: %v, for key: %s",
		duration,
		r.URL.RequestURI(),
	)

	body := append([]byte(nil), recorder.Body.Bytes()...)
	statusCode := recorder.Code

	if errorHolder.err != nil {
		return body, statusCode, errorHolder.err
	}

	if ctx.Err() != nil {
		return body, http.StatusGatewayTimeout, ctx.Err()
	}

	return body, statusCode, nil
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
		http.Error(w, "Backend request failed", http.StatusBadGateway)
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

func (h *Handler) handleBackendSingleFlight(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	ttlL1 time.Duration,
	ttlL2 time.Duration,
) {
	call, isLeader := h.getOrCreateInFlight(key)

	if !isLeader {
		select {
		case <-call.done:
			if call.err != nil {
				http.Error(w, "Backend request failed", http.StatusBadGateway)
				return
			}

			w.WriteHeader(call.statusCode)
			_, _ = w.Write(call.data)

		case <-r.Context().Done():
			http.Error(w, "Request timed out while waiting", http.StatusGatewayTimeout)
		}

		return
	}

	// This request is the first and calls backend
	var (
		body       []byte
		statusCode int
		err        error
	)

	defer func() {
		h.finishInFlight(
			key,
			call,
			body,
			statusCode,
			err,
		)
	}()

	body, statusCode, err = h.fetchFromBackend(r)

	if err != nil {
		http.Error(w, "Backend request failed", http.StatusBadGateway)
		return
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(body)

	if statusCode < 200 || statusCode >= 300 {
		return
	}

	h.setL2(r.Context(), key, body, ttlL2)

	h.setL1(key, KeyValue{
		Value:      append([]byte(nil), body...),
		Expiration: time.Now().Add(ttlL1),
	})

}
