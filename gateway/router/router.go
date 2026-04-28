package router

import (
	"gateway/handlers"

	"github.com/gorilla/mux"
)

func NewRouter(Handler *handlers.Handler) *mux.Router {
	r := mux.NewRouter()

	api := r.PathPrefix("/api").Subrouter()

	RegisterBackendRoutes(api, Handler)

	return r
}
