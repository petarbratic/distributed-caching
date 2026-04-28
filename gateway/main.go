package main

import (
	"gateway/handlers"
	"gateway/router"
	"log"
	"net/http"
	"os"
)

func main() {

	backendURL := os.Getenv("BACKEND_URL")

	if backendURL == "" {
		backendURL = "http://localhost:8081"
	}

	handler, err := handlers.NewHandler(backendURL)

	if err != nil {
		log.Fatalf("Failed to configure backend proxy: %v", err)
	}

	r := router.NewRouter(handler)

	log.Println("Gateway running on: 8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
