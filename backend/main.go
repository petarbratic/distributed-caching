package main

import (
	"backend/handler"
	"backend/service"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"log"
	"net/http"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		req *http.Request,
	) {
		writer.Header().Set(
			"Access-Control-Allow-Origin",
			"http://localhost:4200",
		)
		writer.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, PUT, OPTIONS",
		)
		writer.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type",
		)

		if req.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(writer, req)
	})
}

func main() {

	service := &service.Service{}
	backendHandler := &handler.Handler{
		Service:           service,
		Semaphore:         make(chan struct{}, 15),
		ConcurrentDelayMs: 5,
		BaseLatencyMs:     200,
	}

	handler.RegisterMetrics()

	r := mux.NewRouter()
	r.HandleFunc("/load", backendHandler.GetLoadSignals).Methods("GET")
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/config", backendHandler.GetConfig).Methods("GET")
	r.HandleFunc("/config", backendHandler.UpdateConfig).Methods("PUT")
	r.HandleFunc("/{id}", backendHandler.Get).Methods("GET")

	if err := http.ListenAndServe(
		":8081",
		corsMiddleware(r),
	); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
