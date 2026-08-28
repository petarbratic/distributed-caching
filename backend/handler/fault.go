package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type FaultRequest struct {
	DurationMs int `json:"durationMs"`
}

func (handler *Handler) ActivateFault(w http.ResponseWriter, r *http.Request) {
	var faultRequest FaultRequest

	if err := json.NewDecoder(r.Body).
		Decode(&faultRequest); err != nil {

		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if faultRequest.DurationMs <= 0 {
		http.Error(w, "Duration must be greater than zero", http.StatusBadRequest)
		return
	}

	duration := time.Duration(faultRequest.DurationMs) * time.Millisecond

	handler.Unavailable.Store(true)

	log.Printf("Backend fault activated: duration=%v", duration)

	go func() {
		time.Sleep(duration)

		handler.Unavailable.Store(false)

		log.Printf("Backend fault ended: backend is available now")
	}()

	w.WriteHeader(http.StatusNoContent)
}
