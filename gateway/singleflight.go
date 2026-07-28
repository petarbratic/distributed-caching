package main

import (
	"net/http"
	"time"
)

type inFlightCall struct {
	done       chan struct{}
	data       []byte
	statusCode int
	err        error
}

func (h *Handler) handleBackendSingleFlight(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	ttlL1 time.Duration,
	ttlL2 time.Duration,
	config GatewayConfig,
) {
	call, isLeader := h.getOrCreateInFlight(key)

	if !isLeader {
		singleFlightWaitingRequests.Inc()
		defer singleFlightWaitingRequests.Dec()
		select {
		case <-call.done:
			if call.err != nil {
				http.Error(w, "Backend request failed", http.StatusBadGateway)
				return
			}

			w.WriteHeader(call.statusCode)
			_, _ = w.Write(call.data)

		case <-r.Context().Done():
			http.Error(w, "Request timed out while waiting", http.StatusGatewayTimeout)
		}

		return
	}

	// This request is the first and calls backend
	var (
		value      KeyValue
		body       []byte
		statusCode int
		err        error
	)

	defer func() {
		h.finishInFlight(key, call, body, statusCode, err)
	}()

	if config.DistributedLockEnabled {
		value, statusCode, err = h.fetchWithDistributedLock(r, key, ttlL2, config)
	} else {
		value, statusCode, err = h.fetchFromBackend(r, config.BackendTimeoutMs)
	}

	body = append(
		[]byte(nil),
		value.Value...,
	)

	if err != nil {
		http.Error(w, "Backend request failed", http.StatusBadGateway)
		return
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(body)

	if statusCode < 200 || statusCode >= 300 {
		return
	}
	if !config.DistributedLockEnabled {
		value.L2Expiration = time.Now().Add(ttlL2)

		h.setL2(r.Context(), key, value, ttlL2, config.L2MaxEntries)
	}

	value.L1Expiration = time.Now().Add(ttlL1)

	h.setL1(
		key,
		value,
		config.L1MaxEntries,
	)

}

func (h *Handler) getOrCreateInFlight(key string) (*inFlightCall, bool) {
	h.inFlightMu.Lock()
	defer h.inFlightMu.Unlock()

	if call, exists := h.inFlight[key]; exists {
		return call, false
	}

	call := &inFlightCall{
		done: make(chan struct{}),
	}

	h.inFlight[key] = call

	return call, true
}

func (h *Handler) finishInFlight(
	key string,
	call *inFlightCall,
	data []byte,
	statusCode int,
	err error,
) {
	h.inFlightMu.Lock()
	defer h.inFlightMu.Unlock()

	call.data = append([]byte(nil), data...)
	call.statusCode = statusCode
	call.err = err

	delete(h.inFlight, key)
	close(call.done)
}
