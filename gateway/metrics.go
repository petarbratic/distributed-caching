package main

import "github.com/prometheus/client_golang/prometheus"

func registerMetrics() {
	prometheus.MustRegister(totalUserRequests)
	prometheus.MustRegister(totalFailedRequests)
	prometheus.MustRegister(totalL1Hits)
	prometheus.MustRegister(totalL2Hits)
	prometheus.MustRegister(totalCacheMisses)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(backendDuration)
	prometheus.MustRegister(singleFlightWaitingRequests)
	prometheus.MustRegister(distributedLockAttempts)
	prometheus.MustRegister(backendCallsByKey)
	prometheus.MustRegister(logicalRefreshesByKey)
	prometheus.MustRegister(adaptiveTTLFactor)
	prometheus.MustRegister(adaptiveTTLEffectiveSeconds)
	prometheus.MustRegister(adaptiveTTLState)
	prometheus.MustRegister(adaptiveTTLBackendP99Milliseconds)
}

var totalUserRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_http_requests_total",
		Help: "Total number of HTTP requests gateway received.",
	},
)

var totalFailedRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gateway_failed_requests_total",
		Help: "Total number of failed gateway requests",
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

var singleFlightWaitingRequests = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "gateway_singleflight_waiting_requests",
		Help: "Current number of requests waiting for a SingleFlight result.",
	},
)

var distributedLockAttempts = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_distributed_lock_attempts_total",
		Help: "Total number of distributed lock attempts by result.",
	},
	[]string{"result"},
)

var backendCallsByKey = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_backend_calls_total",
		Help: "Total number of backend calls by cache key.",
	},
	[]string{"key"},
)

var logicalRefreshesByKey = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_logical_refreshes_total",
		Help: "Total number of logical cache refreshes by cache key.",
	},
	[]string{"key"},
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
			0.75,
			1,
			1.25,
			1.5,
			1.75,
			2,
			3,
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
			0.75,
			1,
			1.25,
			1.5,
			1.75,
			2,
			3,
			5,
		},
	},
)

var adaptiveTTLFactor = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "gateway_adaptive_ttl_factor",
		Help: "Current global adaptive TTL scaling factor.",
	},
)

var adaptiveTTLEffectiveSeconds = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "gateway_adaptive_ttl_effective_seconds",
		Help: "Current effective cache TTL in seconds by cache level.",
	},
	[]string{"level"},
)

var adaptiveTTLState = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "gateway_adaptive_ttl_state",
		Help: "Current adaptive TTL controller state: 0=disabled, 1=stable, 2=warning, 3=congested.",
	},
)

var adaptiveTTLBackendP99Milliseconds = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "gateway_adaptive_ttl_backend_p99_milliseconds",
		Help: "Latest backend P99 latency observed by the adaptive TTL controller in milliseconds.",
	},
)
