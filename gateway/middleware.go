package main

import (
	"log"
	"net/http"
)

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		req *http.Request,
	) {
		if req.URL.Path == "/metrics" {
			next.ServeHTTP(writer, req)
			return
		}

		responseWriter := &ResponseWriter{
			ResponseWriter: writer,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(responseWriter, req)

		if responseWriter.statusCode >= 500 {
			log.Println("Failed request")
			totalFailedRequests.Inc()
		}
	})
}
