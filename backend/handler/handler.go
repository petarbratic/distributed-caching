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

	Semaphore   chan struct{}
	semaphoreMu sync.RWMutex

	Active int64
}

func (handler *Handler) Get(writer http.ResponseWriter, req *http.Request) {

	handler.semaphoreMu.RLock()
	semaphore := handler.Semaphore
	handler.semaphoreMu.RUnlock()

	semaphore <- struct{}{}
	defer func() {
		<-semaphore
	}()

	active := atomic.AddInt64(&handler.Active, 1)
	defer atomic.AddInt64(&handler.Active, -1)

	delay := time.Duration(active) * 200 * time.Millisecond
	time.Sleep(delay)

	id := mux.Vars(req)["id"]
	log.Printf("Entity with id %s", id)

	entity, err := handler.Service.FindEntity(id)

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

func (handler *Handler) UpdateSemaphore(
	writer http.ResponseWriter,
	req *http.Request,
) {
	var config SemaphoreConfig

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

	handler.semaphoreMu.Lock()
	handler.Semaphore = make(
		chan struct{},
		config.SemaphoreSize,
	)
	handler.semaphoreMu.Unlock()

	log.Printf(
		"Semaphore size changed to %d",
		config.SemaphoreSize,
	)

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(writer).Encode(config); err != nil {
		log.Println("Failed to encode semaphore config:", err)
	}
}
