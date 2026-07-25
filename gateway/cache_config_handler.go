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
	config, err := h.currentGatewayConfig(r.Context())
	if err != nil {
		log.Println("Failed to read gateway config:", err)

		http.Error(
			w,
			"Failed to read gateway configuration",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(config); err != nil {
		log.Println("Gateway config encoding error:", err)
	}
}

func (h *Handler) updateCacheConfig(
	w http.ResponseWriter,
	r *http.Request,
) {
	var newConfig GatewayConfig

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

	if newConfig.L1TTLSeconds <= 0 {
		http.Error(
			w,
			"L1TTLSeconds must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.L2TTLSeconds <= 0 {
		http.Error(
			w,
			"L2TTLSeconds must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.L1TTLSeconds > newConfig.L2TTLSeconds {
		http.Error(
			w,
			"L1TTLSeconds must not be greater than L2TTLSeconds",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.BackendTimeoutMs <= 0 {
		http.Error(
			w,
			"BackendTimeoutMs must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	version, err := h.configStore.Update(
		r.Context(),
		newConfig,
	)
	if err != nil {
		log.Println("Failed to update gateway config:", err)

		http.Error(
			w,
			"Failed to update gateway configuration",
			http.StatusInternalServerError,
		)
		return
	}

	h.configMu.Lock()
	h.gatewayConfig = newConfig
	h.configVersion = version
	h.clearL1()
	h.configMu.Unlock()

	if err := h.clearL2(r.Context()); err != nil {
		log.Println("Failed to clear L2 cache:", err)

		http.Error(
			w,
			"Configuration changed, but clearing L2 failed",
			http.StatusInternalServerError,
		)
		return
	}

	log.Printf(
		"Gateway configuration changed: L1=%d, L2=%d, TTL L1=%d, TTL L2=%d, SingleFlightEnabled=%t, BackendTimeout=%d",
		newConfig.L1MaxEntries,
		newConfig.L2MaxEntries,
		newConfig.L1TTLSeconds,
		newConfig.L2TTLSeconds,
		newConfig.SingleFlightEnabled,
		newConfig.BackendTimeoutMs,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(newConfig); err != nil {
		log.Println("Gateway config encoding error:", err)
	}
}
