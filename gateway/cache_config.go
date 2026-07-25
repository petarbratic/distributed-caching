package main

type GatewayConfig struct {
	L1MaxEntries int `json:"l1MaxEntries"`
	L2MaxEntries int `json:"l2MaxEntries"`

	L1TTLSeconds int `json:"l1TTLSeconds"`
	L2TTLSeconds int `json:"l2TTLSeconds"`

	SingleFlightEnabled bool `json:"singleFlightEnabled"`
	BackendTimeoutMs    int  `json:"backendTimeoutMs"`
}
