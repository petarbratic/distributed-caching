package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
)

func main() {

	prometheus.MustRegister(totalRequests)

	backendURL := os.Getenv("BACKEND_URL")

	handler, err := NewHandler(backendURL)
	if err != nil {
		log.Fatalf("Failed to configure backend proxy: %v", err)
	}

	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())

	api := r.PathPrefix("/api").Subrouter()
	api.PathPrefix("/backend").Handler(handler)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:4200"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	log.Println("Gateway running on port 8080")

	if err := http.ListenAndServe(":8080", c.Handler(r)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
