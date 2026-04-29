package router

import (
	"gateway/handlers"

	"github.com/gorilla/mux"
)

func NewRouter(handler *handlers.Handler) *mux.Router {
	r := mux.NewRouter()

	api := r.PathPrefix("/api").Subrouter()

	api.PathPrefix("/backend").Handler(handler)

	return r
}
