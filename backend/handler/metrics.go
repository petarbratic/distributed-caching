package handler

import (
	"github.com/prometheus/client_golang/prometheus"
)

var totalBackendRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backend_requests_total",
		Help: "Total number of HTTP requests sent to backend.",
	},
)

var backendActiveRequests = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "backend_active_requests",
		Help: "Current number of requests being processed concurrently by the backend.",
	},
)

func RegisterMetrics() {
	prometheus.MustRegister(
		totalBackendRequests,
		backendActiveRequests,
	)
}
