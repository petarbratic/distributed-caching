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

	Active int64
}

func (handler *Handler) Get(writer http.ResponseWriter, req *http.Request) {

	handler.configMu.RLock()
	semaphore := handler.Semaphore
	concurrentDelayMs := handler.ConcurrentDelayMs
	baseLatencyMs := handler.BaseLatencyMs
	handler.configMu.RUnlock()

	semaphore <- struct{}{}
	defer func() {
		<-semaphore
	}()

	active := atomic.AddInt64(&handler.Active, 1)
	defer atomic.AddInt64(&handler.Active, -1)

	concurrentDelay :=
		time.Duration(active) *
			time.Duration(concurrentDelayMs) *
			time.Millisecond
	time.Sleep(concurrentDelay)

	id := mux.Vars(req)["id"]
	log.Printf("Entity with id %s", id)

	entity, err := handler.Service.FindEntity(
		id,
		time.Duration(baseLatencyMs)*time.Millisecond,
	)

	writer.Header().Set("Content-Type", "application/json")

	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

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
