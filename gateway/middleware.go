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

func gatewayInstanceMiddleware(
	instanceName string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			writer http.ResponseWriter,
			req *http.Request,
		) {
			writer.Header().Set(
				"X-Gateway-Instance",
				instanceName,
			)

			if req.URL.Path != "/metrics" {
				log.Printf(
					"Gateway instance %s received %s %s",
					instanceName,
					req.Method,
					req.URL.RequestURI(),
				)
			}

			next.ServeHTTP(writer, req)
		})
	}
}
