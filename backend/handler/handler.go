package handler

import (
	"backend/service"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

type Handler struct {
	Service *service.Service

	Semaphore chan struct{}

	ConcurrentDelayMs int
	BaseLatencyMs     int

	configMu sync.RWMutex

	Active      int64
	Unavailable atomic.Bool

	loadSignalsMu    sync.RWMutex
	requestDurations []latencySample
}

func (handler *Handler) Get(writer http.ResponseWriter, req *http.Request) {
	requestStartedAt := time.Now()

	totalBackendRequests.Inc()

	if handler.Unavailable.Load() {
		http.Error(
			writer,
			"Backend temporarily unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}

	atomic.AddInt64(&handler.Active, 1)
	backendActiveRequests.Inc()

	defer func() {
		atomic.AddInt64(&handler.Active, -1)
		backendActiveRequests.Dec()
		handler.recordRequestDuration(time.Since(requestStartedAt))
	}()

	handler.configMu.RLock()

	semaphore := handler.Semaphore
	concurrentDelayMs := handler.ConcurrentDelayMs
	baseLatencyMs := handler.BaseLatencyMs

	handler.configMu.RUnlock()

	select {
	case semaphore <- struct{}{}:
		defer func() {
			<-semaphore
		}()

	case <-req.Context().Done():
		log.Printf("Request canceled while waiting for backend semaphore")
		return
	}

	processingRequests := len(semaphore)

	concurrentDelay :=
		time.Duration(processingRequests) *
			time.Duration(concurrentDelayMs) *
			time.Millisecond

	select {
	case <-time.After(concurrentDelay):

	case <-req.Context().Done():
		log.Printf("Request canceled during concurrent delay")
		return
	}

	id := mux.Vars(req)["id"]
	//log.Printf("Entity with id %s", id)

	entity, err := handler.Service.FindEntity(
		req.Context(),
		id,
		time.Duration(baseLatencyMs)*time.Millisecond,
	)

	if err != nil {
		if req.Context().Err() != nil {
			log.Printf("Request canceled while fetching entity: %v", req.Context().Err())
			return
		}

		writer.WriteHeader(http.StatusNotFound)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(writer).Encode(entity); err != nil {
		log.Println("Failed to encode entity:", err)
	}
}

func (handler *Handler) UpdateConfig(
	writer http.ResponseWriter,
	req *http.Request,
) {
	var config BackendConfig

	if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
		http.Error(
			writer,
			"Invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if config.SemaphoreSize <= 0 {
		http.Error(
			writer,
			"Semaphore size must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if config.ConcurrentDelayMs < 0 {
		http.Error(
			writer,
			"Concurrent delay must not be negative",
			http.StatusBadRequest,
		)
		return
	}

	if config.BaseLatencyMs < 0 {
		http.Error(
			writer,
			"Base latency must not be negative",
			http.StatusBadRequest,
		)
		return
	}

	handler.configMu.Lock()

	handler.Semaphore = make(
		chan struct{},
		config.SemaphoreSize,
	)
	handler.ConcurrentDelayMs = config.ConcurrentDelayMs
	handler.BaseLatencyMs = config.BaseLatencyMs

	handler.configMu.Unlock()

	log.Printf(
		"Backend configuration changed: semaphore=%d, concurrentDelay=%dms, baseLatency=%dms",
		config.SemaphoreSize,
		config.ConcurrentDelayMs,
		config.BaseLatencyMs,
	)

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(writer).Encode(config); err != nil {
		log.Println("Failed to encode backend config:", err)
	}
}

func (handler *Handler) GetConfig(
	writer http.ResponseWriter,
	req *http.Request,
) {
	handler.configMu.RLock()

	config := BackendConfig{
		SemaphoreSize:     cap(handler.Semaphore),
		ConcurrentDelayMs: handler.ConcurrentDelayMs,
		BaseLatencyMs:     handler.BaseLatencyMs,
	}

	handler.configMu.RUnlock()

	writer.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(writer).Encode(config); err != nil {
		log.Println("Failed to encode backend config:", err)
	}
}
