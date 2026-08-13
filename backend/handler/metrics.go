package handler

import (
	"github.com/prometheus/client_golang/prometheus"
)

var totalBackendRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backend_requests_total",
		Help: "Total number of requests received by the backend, including requests canceled while waiting for the semaphore.",
	},
)

var backendActiveRequests = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "backend_active_requests",
		Help: "Current number of backend requests being processed or waiting for the semaphore.",
	},
)

func RegisterMetrics() {
	prometheus.MustRegister(
		totalBackendRequests,
		backendActiveRequests,
	)
}
