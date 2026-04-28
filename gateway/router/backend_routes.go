package router

import (
	"gateway/handlers"
	"github.com/gorilla/mux"
)

func RegisterBackendRoutes(api *mux.Router, handler *handlers.Handler) {
	api.PathPrefix("/backend").Handler(handler)
}
