package handler

import (
	"backend/service"
	"encoding/json"
	//"github.com/google/uuid"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type Handler struct {
	Service *service.Service
}

func (handler *Handler) Get(writer http.ResponseWriter, req *http.Request) {

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
