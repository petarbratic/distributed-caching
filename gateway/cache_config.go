package main

const (
	defaultAdaptiveTTLMinFactor            = 0.75
	defaultAdaptiveTTLMaxFactor            = 4.0
	defaultAdaptiveTTLLatencyThresholdMs   = 500
	defaultAdaptiveTTLConcurrencyThreshold = 12
	defaultAdaptiveTTLAdjustmentIntervalMs = 5000
)

type GatewayConfig struct {
	L1MaxEntries int `json:"l1MaxEntries"`
	L2MaxEntries int `json:"l2MaxEntries"`

	L1TTLSeconds int `json:"l1TTLSeconds"`
	L2TTLSeconds int `json:"l2TTLSeconds"`

	SingleFlightEnabled              bool    `json:"singleFlightEnabled"`
	DistributedLockEnabled           bool    `json:"distributedLockEnabled"`
	ProbabilisticEarlyRefreshEnabled bool    `json:"probabilisticEarlyRefreshEnabled"`
	EarlyRefreshBeta                 float64 `json:"earlyRefreshBeta"`
	BackendTimeoutMs                 int     `json:"backendTimeoutMs"`

	AdaptiveTTLEnabled              bool    `json:"adaptiveTTLEnabled"`
	AdaptiveTTLMinFactor            float64 `json:"adaptiveTTLMinFactor"`
	AdaptiveTTLMaxFactor            float64 `json:"adaptiveTTLMaxFactor"`
	AdaptiveTTLLatencyThresholdMs   int     `json:"adaptiveTTLLatencyThresholdMs"`
	AdaptiveTTLConcurrencyThreshold int     `json:"adaptiveTTLConcurrencyThreshold"`
	AdaptiveTTLAdjustmentIntervalMs int     `json:"adaptiveTTLAdjustmentIntervalMs"`
}

func applyAdaptiveTTLDefaults(config GatewayConfig) GatewayConfig {
	if config.AdaptiveTTLMinFactor <= 0 {
		config.AdaptiveTTLMinFactor = defaultAdaptiveTTLMinFactor
	}

	if config.AdaptiveTTLMaxFactor <= 0 {
		config.AdaptiveTTLMaxFactor = defaultAdaptiveTTLMaxFactor
	}

	if config.AdaptiveTTLLatencyThresholdMs <= 0 {
		config.AdaptiveTTLLatencyThresholdMs = defaultAdaptiveTTLLatencyThresholdMs
	}

	if config.AdaptiveTTLConcurrencyThreshold <= 0 {
		config.AdaptiveTTLConcurrencyThreshold = defaultAdaptiveTTLConcurrencyThreshold
	}

	if config.AdaptiveTTLAdjustmentIntervalMs <= 0 {
		config.AdaptiveTTLAdjustmentIntervalMs = defaultAdaptiveTTLAdjustmentIntervalMs
	}

	return config
}
