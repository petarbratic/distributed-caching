package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func (h *Handler) getCacheConfig(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(h.cacheConfig); err != nil {
		log.Println("Cache config encoding error: ", err)
		http.Error(
			w,
			"Failed to encode cache configuration",
			http.StatusInternalServerError,
		)
	}
}

func (h *Handler) updateCacheConfig(
	w http.ResponseWriter,
	r *http.Request,
) {
	var newConfig CacheConfig

	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(
			w,
			"Invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.L1MaxEntries <= 0 {
		http.Error(
			w,
			"L1MaxEntries must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.L2MaxEntries <= 0 {
		http.Error(
			w,
			"L2MaxEntries must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.L1MaxEntries > newConfig.L2MaxEntries {
		http.Error(
			w,
			"L1MaxEntries must not be greater than L2MaxEntries",
			http.StatusBadRequest,
		)
		return
	}

	if err := h.updateBackendConfig(
		r.Context(),
		newConfig.SemaphoreSize,
		newConfig.ConcurrentDelayMs,
		newConfig.BaseLatencyMs,
	); err != nil {
		log.Println("Failed to update backend config:", err)

		http.Error(
			w,
			"Failed to update backend config",
			http.StatusBadGateway,
		)
		return
	}

	h.cacheConfig = newConfig

	h.clearL1()

	if err := h.clearL2(r.Context()); err != nil {
		log.Println("Failed to clear L2 cache: ", err)

		http.Error(
			w,
			"Configuration changed, but clearing L2 failed",
			http.StatusInternalServerError,
		)
		return
	}

	log.Printf(
		"Cache configuration changed: L1=%d, L2=%d, TTL L1=%d, TTL L2=%d, Semaphore size = %d, Concurent=%d, Base=%d, SingleFlightEnabled=%t, BackendTimeout=%d",
		newConfig.L1MaxEntries,
		newConfig.L2MaxEntries,
		newConfig.L1TTLSeconds,
		newConfig.L2TTLSeconds,
		newConfig.SemaphoreSize,
		newConfig.ConcurrentDelayMs,
		newConfig.BaseLatencyMs,
		newConfig.SingleFlightEnabled,
		newConfig.BackendTimeoutMs,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(h.cacheConfig); err != nil {
		log.Println("Cache config encoding error: ", err)
	}
}
