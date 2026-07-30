package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const adaptiveTTLInitialFactor = 1.0

type AdaptiveTTLState string

const (
	AdaptiveTTLStateDisabled  AdaptiveTTLState = "disabled"
	AdaptiveTTLStateStable    AdaptiveTTLState = "stable"
	AdaptiveTTLStateWarning   AdaptiveTTLState = "warning"
	AdaptiveTTLStateCongested AdaptiveTTLState = "congested"
)

type BackendLoadSignals struct {
	P99LatencyMs          float64 `json:"p99LatencyMs"`
	ActiveRequests        int64   `json:"activeRequests"`
	MaxConcurrentRequests int     `json:"maxConcurrentRequests"`
	Utilization           float64 `json:"utilization"`
	SampleCount           int     `json:"sampleCount"`
}

type AdaptiveTTLSnapshot struct {
	Factor float64
	State  AdaptiveTTLState
}

type AdaptiveTTLController struct {
	backendLoadURL string
	httpClient     *http.Client
	redis          *redis.Client

	mu sync.RWMutex

	currentFactor float64
	currentState  AdaptiveTTLState

	consecutiveStableReadings    int
	consecutiveCongestedReadings int
}

func NewAdaptiveTTLController(backendURL string, redisClient *redis.Client) (*AdaptiveTTLController, error) {

	backendLoadURL, err := url.JoinPath(backendURL, "/load")

	if err != nil {
		return nil, fmt.Errorf("failed to construct backend load URL: %w", err)
	}

	return &AdaptiveTTLController{
		backendLoadURL: backendLoadURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		redis:         redisClient,
		currentFactor: adaptiveTTLInitialFactor,
		currentState:  AdaptiveTTLStateDisabled,
	}, nil
}

func (controller *AdaptiveTTLController) Snapshot() AdaptiveTTLSnapshot {
	controller.mu.RLock()
	defer controller.mu.RUnlock()

	return AdaptiveTTLSnapshot{
		Factor: controller.currentFactor,
		State:  controller.currentState,
	}
}
