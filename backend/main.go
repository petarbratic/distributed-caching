package main

import (
	"backend/handler"
	"backend/service"

	"github.com/gorilla/mux"

	"log"
	"net/http"
)

func main() {

	service := &service.Service{}
	handler := &handler.Handler{
		Service:           service,
		Semaphore:         make(chan struct{}, 3),
		ConcurrentDelayMs: 200,
		BaseLatencyMs:     2000,
	}

	r := mux.NewRouter()
	r.HandleFunc("/{id}", handler.Get).Methods("GET")
	r.HandleFunc("/config", handler.UpdateConfig).Methods("PUT")

	log.Println("Backend running on: 8081!")
	if err := http.ListenAndServe(":8081", r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
