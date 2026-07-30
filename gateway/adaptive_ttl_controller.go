package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	adaptiveTTLInitialFactor = 1.0

	adaptiveTTLMinimumSampleCount = 20

	adaptiveTTLCongestedReadingsRequired = 2
	adaptiveTTLStableReadingsRequired    = 3

	adaptiveTTLStableThresholdRatio = 0.8

	adaptiveTTLIncreaseStep       = 0.25
	adaptiveTTLStrongIncreaseStep = 0.5
	adaptiveTTLDecreaseStep       = 0.1

	adaptiveTTLFallbackInterval = 5 * time.Second
)

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

type GatewayConfigProvider func(context.Context) (GatewayConfig, error)

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

func (controller *AdaptiveTTLController) Run(ctx context.Context, configProvider GatewayConfigProvider) {
	log.Printf("Adaptive TTL controller started: backendLoadURL=%s", controller.backendLoadURL)

	for {
		config, err := configProvider(ctx)
		if err != nil {
			log.Printf("Adaptive TTL failed to read gateway configuration: %v", err)

			if !waitForAdaptiveTTLInterval(ctx, adaptiveTTLFallbackInterval) {
				return
			}
			continue
		}

		interval := time.Duration(config.AdaptiveTTLAdjustmentIntervalMs) * time.Millisecond

		if interval <= 0 {
			interval = adaptiveTTLFallbackInterval
		}

		if !config.AdaptiveTTLEnabled {
			controller.setDisabled()

			if !waitForAdaptiveTTLInterval(ctx, interval) {
				return
			}

			continue
		}

		signals, err := controller.readBackendLoadSignals(ctx)
		if err != nil {
			log.Printf("Adaptive TTL failed to read backend load signals: %v", err)

			if !waitForAdaptiveTTLInterval(ctx, interval) {
				return
			}

			continue
		}

		controller.processSignals(config, signals)

		if !waitForAdaptiveTTLInterval(ctx, interval) {
			return
		}
	}
}

func (controller *AdaptiveTTLController) readBackendLoadSignals(ctx context.Context) (BackendLoadSignals, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, controller.backendLoadURL, nil)

	if err != nil {
		return BackendLoadSignals{}, fmt.Errorf("failed to create backend load request: %w", err)
	}

	response, err := controller.httpClient.Do(request)
	if err != nil {
		return BackendLoadSignals{}, fmt.Errorf("backend load request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return BackendLoadSignals{}, fmt.Errorf("backend load endpoint returned status %d", response.StatusCode)
	}

	var signals BackendLoadSignals

	if err := json.NewDecoder(response.Body).Decode(&signals); err != nil {
		return BackendLoadSignals{}, fmt.Errorf("failed to decode backend load signals: %w", err)
	}

	return signals, nil
}

func (controller *AdaptiveTTLController) processSignals(config GatewayConfig, signals BackendLoadSignals) {
	state, latencyCongested, concurrencyCongested := classifyBackendLoad(config, signals)

	controller.mu.Lock()

	previousState := controller.currentState
	previousFactor := controller.currentFactor

	controller.currentFactor = clampAdaptiveTTLFactor(controller.currentFactor, config.AdaptiveTTLMinFactor, config.AdaptiveTTLMaxFactor)

	controller.currentState = state

	switch state {
	case AdaptiveTTLStateCongested:
		controller.consecutiveStableReadings = 0
		controller.consecutiveCongestedReadings++

		if controller.consecutiveCongestedReadings >= adaptiveTTLCongestedReadingsRequired {
			increaseStep := adaptiveTTLIncreaseStep

			if latencyCongested && concurrencyCongested {
				increaseStep = adaptiveTTLStrongIncreaseStep
			}

			controller.currentFactor =
				clampAdaptiveTTLFactor(
					controller.currentFactor+increaseStep,
					config.AdaptiveTTLMinFactor,
					config.AdaptiveTTLMaxFactor,
				)

			controller.consecutiveCongestedReadings = 0
		}

	case AdaptiveTTLStateStable:
		controller.consecutiveCongestedReadings = 0
		controller.consecutiveStableReadings++

		if controller.consecutiveStableReadings >= adaptiveTTLStableReadingsRequired {
			controller.currentFactor =
				clampAdaptiveTTLFactor(
					controller.currentFactor-adaptiveTTLDecreaseStep,
					config.AdaptiveTTLMinFactor,
					config.AdaptiveTTLMaxFactor,
				)

			controller.consecutiveStableReadings = 0
		}

	case AdaptiveTTLStateWarning:
		controller.consecutiveStableReadings = 0
		controller.consecutiveCongestedReadings = 0
	}

	currentState := controller.currentState
	currentFactor := controller.currentFactor

	controller.mu.Unlock()

	if previousState != currentState {
		log.Printf(
			"Adaptive TTL state changed: %s -> %s; p99=%.2fms, active=%d/%d, samples=%d",
			previousState,
			currentState,
			signals.P99LatencyMs,
			signals.ActiveRequests,
			signals.MaxConcurrentRequests,
			signals.SampleCount,
		)
	}

	if previousFactor != currentFactor {
		log.Printf(
			"Adaptive TTL factor changed: %.2f -> %.2f; state=%s, p99=%.2fms, active=%d/%d",
			previousFactor,
			currentFactor,
			currentState,
			signals.P99LatencyMs,
			signals.ActiveRequests,
			signals.MaxConcurrentRequests,
		)
	}
}

func classifyBackendLoad(config GatewayConfig, signals BackendLoadSignals) (AdaptiveTTLState, bool, bool) {
	enoughLatencySamples := signals.SampleCount >= adaptiveTTLMinimumSampleCount

	latencyCongested := enoughLatencySamples && signals.P99LatencyMs >= float64(config.AdaptiveTTLLatencyThresholdMs)

	concurrencyCongested := signals.ActiveRequests >= int64(config.AdaptiveTTLConcurrencyThreshold)

	if latencyCongested || concurrencyCongested {
		return AdaptiveTTLStateCongested, latencyCongested, concurrencyCongested
	}

	if !enoughLatencySamples {
		return AdaptiveTTLStateWarning, false, false
	}

	stableLatencyThreshold := float64(config.AdaptiveTTLLatencyThresholdMs) * adaptiveTTLStableThresholdRatio

	stableConcurrencyThreshold := float64(config.AdaptiveTTLConcurrencyThreshold) * adaptiveTTLStableThresholdRatio

	if signals.P99LatencyMs < stableLatencyThreshold && float64(signals.ActiveRequests) < stableConcurrencyThreshold {
		return AdaptiveTTLStateStable, false, false
	}

	return AdaptiveTTLStateWarning, false, false
}

func (controller *AdaptiveTTLController) setDisabled() {
	controller.mu.Lock()

	previousState := controller.currentState
	previousFactor := controller.currentFactor

	controller.currentState = AdaptiveTTLStateDisabled
	controller.currentFactor = adaptiveTTLInitialFactor
	controller.consecutiveStableReadings = 0
	controller.consecutiveCongestedReadings = 0

	controller.mu.Unlock()

	if previousState != AdaptiveTTLStateDisabled || previousFactor != adaptiveTTLInitialFactor {
		log.Printf(
			"Adaptive TTL disabled: state=%s -> %s, factor=%.2f -> %.2f",
			previousState,
			AdaptiveTTLStateDisabled,
			previousFactor,
			adaptiveTTLInitialFactor,
		)
	}
}

func clampAdaptiveTTLFactor(factor float64, minFactor float64, maxFactor float64) float64 {
	if factor < minFactor {
		return minFactor
	}

	if factor > maxFactor {
		return maxFactor
	}

	return factor
}

func waitForAdaptiveTTLInterval(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
