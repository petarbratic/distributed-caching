package main

import (
	"backend/handler"
	"backend/service"

	"github.com/gorilla/mux"
	//"gorm.io/driver/postgres"
	//"gorm.io/gorm"
	"log"
	"net/http"
	//"os"
)

func main() {

	service := &service.Service{}
	handler := &handler.Handler{Service: service}

	r := mux.NewRouter()
	r.HandleFunc("/{id}", handler.Get).Methods("GET")

	log.Println("Backend running on: 8081!")
	if err := http.ListenAndServe(":8081", r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
