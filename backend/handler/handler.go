package handler

import (
	"backend/service"
	"encoding/json"
	"sync/atomic"
	"time"

	//"github.com/google/uuid"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type Handler struct {
	Service   *service.Service
	Semaphore chan struct{}
	Active    int64
}

func (handler *Handler) Get(writer http.ResponseWriter, req *http.Request) {

	handler.Semaphore <- struct{}{}
	defer func() {
		<-handler.Semaphore
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
	json.NewEncoder(writer).Encode(entity)
}
