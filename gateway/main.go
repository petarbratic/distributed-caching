package main

import (
	"gateway/handlers"
	"gateway/router"
	"github.com/rs/cors"
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

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:4200"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	log.Println("Gateway running on: 8080")

	if err := http.ListenAndServe(":8080", c.Handler(r)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
