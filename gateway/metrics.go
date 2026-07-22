package main

import "github.com/prometheus/client_golang/prometheus"

var totalUserRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_http_requests_total",
		Help: "Total number of HTTP requests gateway received.",
	},
)

var totalBackendRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_backend_requests_total",
		Help: "Total number of HTTP requests sent to backend.",
	},
)

var totalL1Hits = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_l1_hits_total",
		Help: "Total number of L1 hits.",
	},
)

var totalL2Hits = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_l2_hits_total",
		Help: "Total number of L2 hits.",
	},
)

var totalCacheMisses = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_cache_miss_total",
		Help: "Total number of cache misses.",
	},
)

var requestDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name: "gateway_request_duration_seconds",
		Help: "Total duration of processing a user request (in seconds).",
		Buckets: []float64{
			0.001,
			0.005,
			0.01,
			0.025,
			0.05,
			0.1,
			0.25,
			0.5,
			1,
			2,
			5,
		},
	},
)

var backendDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name: "gateway_backend_duration_seconds",
		Help: "Total duration of backend processing a user request (in seconds).",
		Buckets: []float64{
			0.001,
			0.005,
			0.01,
			0.025,
			0.05,
			0.1,
			0.25,
			0.5,
			1,
			2,
			5,
		},
	},
)
