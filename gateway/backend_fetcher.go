package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"
)

type proxyErrorHolder struct {
	err error
}

type proxyErrorContextKey struct{}

func (h *Handler) fetchFromBackend(r *http.Request, backendTimeoutMs int) (KeyValue, int, error) {

	backendStart := time.Now()

	timeout := time.Duration(backendTimeoutMs) * time.Millisecond
	h.recordBackendCallForFAF(r.Context(), r.URL.RequestURI(), timeout)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	errorHolder := &proxyErrorHolder{}

	ctx = context.WithValue(
		ctx,
		proxyErrorContextKey{},
		errorHolder,
	)

	requestCopy := r.Clone(ctx)
	recorder := httptest.NewRecorder()

	h.proxy.ServeHTTP(recorder, requestCopy)

	duration := time.Since(backendStart)
	backendDuration.Observe(duration.Seconds())

	//log.Printf("Backend call duration: %v, for key: %s", duration, r.URL.RequestURI())

	result := KeyValue{
		Value: append(
			[]byte(nil),
			recorder.Body.Bytes()...,
		),
		RecomputeDuration: duration,
	}

	statusCode := recorder.Code

	if errorHolder.err != nil {
		return result, statusCode, errorHolder.err
	}

	if ctx.Err() != nil {
		return result, http.StatusGatewayTimeout, ctx.Err()
	}

	return result, statusCode, nil
}
