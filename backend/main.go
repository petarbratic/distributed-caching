package main

import (
	"backend/handler"
	"backend/service"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"log"
	"net/http"
)

func main() {

	service := &service.Service{}
	backendHandler := &handler.Handler{
		Service:           service,
		Semaphore:         make(chan struct{}, 3),
		ConcurrentDelayMs: 200,
		BaseLatencyMs:     2000,
	}

	handler.RegisterMetrics()

	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/{id}", backendHandler.Get).Methods("GET")
	r.HandleFunc("/config", backendHandler.UpdateConfig).Methods("PUT")

	log.Println("Backend running on: 8081!")
	if err := http.ListenAndServe(":8081", r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
