package handler

type BackendConfig struct {
	SemaphoreSize     int `json:"semaphoreSize"`
	ConcurrentDelayMs int `json:"concurrentDelayMs"`
	BaseLatencyMs     int `json:"baseLatencyMs"`
}
