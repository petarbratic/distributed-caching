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

	if newConfig.EarlyRefreshBeta < 0.1 ||
		newConfig.EarlyRefreshBeta > 10 {
		http.Error(
			w,
			"EarlyRefreshBeta must be between 0.1 and 10",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLMinFactor <= 0 {
		http.Error(
			w,
			"AdaptiveTTLMinFactor must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLMinFactor >
		adaptiveTTLInitialFactor {
		http.Error(
			w,
			"AdaptiveTTLMinFactor must not be greater than 1",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLMaxFactor <
		adaptiveTTLInitialFactor {
		http.Error(
			w,
			"AdaptiveTTLMaxFactor must not be smaller than 1",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLMaxFactor <
		newConfig.AdaptiveTTLMinFactor {
		http.Error(
			w,
			"AdaptiveTTLMaxFactor must not be smaller than AdaptiveTTLMinFactor",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLLatencyThresholdMs <= 0 {
		http.Error(
			w,
			"AdaptiveTTLLatencyThresholdMs must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLConcurrencyThreshold <= 0 {
		http.Error(
			w,
			"AdaptiveTTLConcurrencyThreshold must be greater than zero",
			http.StatusBadRequest,
		)
		return
	}

	if newConfig.AdaptiveTTLAdjustmentIntervalMs <= 0 {
		http.Error(
			w,
			"AdaptiveTTLAdjustmentIntervalMs must be greater than zero",
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

	if err := h.adaptiveTTLController.Reset(r.Context(), newConfig.AdaptiveTTLEnabled); err != nil {
		log.Println("Failed to reset adaptive TTL controller:", err)

		http.Error(w, "Configuration changed, but resetting adaptive TTL failed", http.StatusInternalServerError)
		return
	}

	log.Printf(
		"Gateway configuration changed: L1=%d, L2=%d, TTL L1=%d, TTL L2=%d, SingleFlightEnabled=%t, DistributedLockEnabled=%t, ProbabilisticEarlyRefreshEnabled=%t, EarlyRefreshBeta=%.2f, BackendTimeout=%d, AdaptiveTTLEnabled=%t, AdaptiveTTLMinFactor=%.2f, AdaptiveTTLMaxFactor=%.2f, AdaptiveTTLLatencyThreshold=%dms, AdaptiveTTLConcurrencyThreshold=%d, AdaptiveTTLAdjustmentInterval=%dms",
		newConfig.L1MaxEntries,
		newConfig.L2MaxEntries,
		newConfig.L1TTLSeconds,
		newConfig.L2TTLSeconds,
		newConfig.SingleFlightEnabled,
		newConfig.DistributedLockEnabled,
		newConfig.ProbabilisticEarlyRefreshEnabled,
		newConfig.EarlyRefreshBeta,
		newConfig.BackendTimeoutMs,
		newConfig.AdaptiveTTLEnabled,
		newConfig.AdaptiveTTLMinFactor,
		newConfig.AdaptiveTTLMaxFactor,
		newConfig.AdaptiveTTLLatencyThresholdMs,
		newConfig.AdaptiveTTLConcurrencyThreshold,
		newConfig.AdaptiveTTLAdjustmentIntervalMs,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(newConfig); err != nil {
		log.Println("Gateway config encoding error:", err)
	}
}
