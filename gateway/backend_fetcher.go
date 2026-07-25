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

func (h *Handler) fetchFromBackend(r *http.Request) ([]byte, int, error) {

	backendStart := time.Now()

	timeout := time.Duration(h.cacheConfig.BackendTimeoutMs) * time.Millisecond
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

	body := append([]byte(nil), recorder.Body.Bytes()...)
	statusCode := recorder.Code

	if errorHolder.err != nil {
		return body, statusCode, errorHolder.err
	}

	if ctx.Err() != nil {
		return body, http.StatusGatewayTimeout, ctx.Err()
	}

	return body, statusCode, nil
}
