package main

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"log"
	"net/http"
	"os"
)

func main() {

	registerMetrics()

	backendURL := os.Getenv("BACKEND_URL")

	instanceName := os.Getenv("INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "gateway"
	}

	handler, err := NewHandler(backendURL)
	if err != nil {
		log.Fatalf("Failed to configure backend proxy: %v", err)
	}

	go handler.adaptiveTTLController.Run(context.Background(), handler.currentGatewayConfig)

	r := mux.NewRouter()

	r.Use(metricsMiddleware)
	r.Use(gatewayInstanceMiddleware(instanceName))

	r.Handle("/metrics", promhttp.Handler())

	api := r.PathPrefix("/api").Subrouter()

	api.PathPrefix("/backend").Handler(handler)

	api.HandleFunc(
		"/cache-config",
		handler.getCacheConfig,
	).Methods(http.MethodGet)

	api.HandleFunc(
		"/cache-config",
		handler.updateCacheConfig,
	).Methods(http.MethodPut)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:4200",
		},
		AllowedMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			"X-Gateway-Instance",
		},
	})

	log.Printf("Gateway instance %s running on port 8080", instanceName)

	if err := http.ListenAndServe(":8080", c.Handler(r)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
