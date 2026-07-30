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

const (
	adaptiveTTLSnapshotRedisKey = "gateway:adaptive-ttl:snapshot"

	adaptiveTTLControllerLockRedisKey = "gateway:adaptive-ttl:controller-lock"

	adaptiveTTLControllerLockTTL = 5 * time.Second

	adaptiveTTLControllerLockReleaseTimeout = time.Second
)

var releaseAdaptiveTTLControllerLockScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end

		return 0
	`)

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
	Factor float64          `json:"factor"`
	State  AdaptiveTTLState `json:"state"`

	ConsecutiveStableReadings int `json:"consecutiveStableReadings"`

	ConsecutiveCongestedReadings int `json:"consecutiveCongestedReadings"`
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

		ConsecutiveStableReadings: controller.consecutiveStableReadings,

		ConsecutiveCongestedReadings: controller.consecutiveCongestedReadings,
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

		if err := controller.updateSharedState(ctx, config); err != nil {
			log.Printf("Adaptive TTL update failed: %v", err)
		}

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

func (controller *AdaptiveTTLController) updateSharedState(ctx context.Context, config GatewayConfig) error {
	token, acquired, err := controller.tryAcquireControllerLock(ctx)
	if err != nil {
		return err
	}

	if !acquired {
		return controller.synchronizeFromRedis(ctx)
	}

	defer controller.releaseControllerLock(token)

	snapshot, err := controller.loadSharedSnapshot(ctx)
	if err != nil {
		return err
	}

	controller.applySnapshot(snapshot)

	if !config.AdaptiveTTLEnabled {
		controller.setDisabled()

		if err := controller.saveSharedSnapshot(ctx, controller.Snapshot()); err != nil {
			return err
		}

		return nil
	}

	signals, err := controller.readBackendLoadSignals(ctx)
	if err != nil {
		return err
	}

	controller.processSignals(config, signals)

	if err := controller.saveSharedSnapshot(ctx, controller.Snapshot()); err != nil {
		return err
	}

	return nil
}

func (controller *AdaptiveTTLController) tryAcquireControllerLock(ctx context.Context) (string, bool, error) {
	token, err := newDistributedLockToken()
	if err != nil {
		return "", false, fmt.Errorf("generate adaptive TTL controller lock token: %w", err)
	}

	acquired, err := controller.redis.SetNX(ctx, adaptiveTTLControllerLockRedisKey, token, adaptiveTTLControllerLockTTL).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire adaptive TTL controller lock: %w", err)
	}

	return token, acquired, nil
}

func (controller *AdaptiveTTLController) releaseControllerLock(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), adaptiveTTLControllerLockReleaseTimeout)
	defer cancel()

	released, err := releaseAdaptiveTTLControllerLockScript.Run(
		ctx,
		controller.redis,
		[]string{
			adaptiveTTLControllerLockRedisKey,
		},
		token,
	).Int64()

	if err != nil {
		log.Printf("Adaptive TTL controller lock release failed: %v", err)
		return
	}

	if released == 0 {
		log.Printf("Adaptive TTL controller lock was not released because ownership changed or the lock expired")
	}
}

func (controller *AdaptiveTTLController) loadSharedSnapshot(ctx context.Context) (AdaptiveTTLSnapshot, error) {
	data, err := controller.redis.Get(ctx, adaptiveTTLSnapshotRedisKey).Bytes()

	if err == redis.Nil {
		initialSnapshot := AdaptiveTTLSnapshot{
			Factor: adaptiveTTLInitialFactor,
			State:  AdaptiveTTLStateDisabled,
		}

		initialData, marshalErr := json.Marshal(initialSnapshot)
		if marshalErr != nil {
			return AdaptiveTTLSnapshot{}, fmt.Errorf("encode initial adaptive TTL snapshot: %w", marshalErr)
		}

		created, setErr := controller.redis.SetNX(ctx, adaptiveTTLSnapshotRedisKey, initialData, 0).Result()
		if setErr != nil {
			return AdaptiveTTLSnapshot{}, fmt.Errorf("initialize adaptive TTL snapshot: %w", setErr)
		}

		if created {
			return initialSnapshot, nil
		}

		data, err = controller.redis.Get(ctx, adaptiveTTLSnapshotRedisKey).Bytes()
	}

	if err != nil {
		return AdaptiveTTLSnapshot{}, fmt.Errorf("read adaptive TTL snapshot: %w", err)
	}

	var snapshot AdaptiveTTLSnapshot

	if err := json.Unmarshal(data, &snapshot); err != nil {
		return AdaptiveTTLSnapshot{}, fmt.Errorf("decode adaptive TTL snapshot: %w", err)
	}

	if snapshot.Factor <= 0 {
		return AdaptiveTTLSnapshot{}, fmt.Errorf("invalid adaptive TTL factor in Redis: %.2f", snapshot.Factor)
	}

	if snapshot.State == "" {
		return AdaptiveTTLSnapshot{}, fmt.Errorf("adaptive TTL state is missing in Redis")
	}

	return snapshot, nil
}

func (controller *AdaptiveTTLController) saveSharedSnapshot(ctx context.Context, snapshot AdaptiveTTLSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode adaptive TTL snapshot: %w", err)
	}

	if err := controller.redis.Set(ctx, adaptiveTTLSnapshotRedisKey, data, 0).Err(); err != nil {
		return fmt.Errorf("save adaptive TTL snapshot: %w", err)
	}

	return nil
}

func (controller *AdaptiveTTLController) synchronizeFromRedis(ctx context.Context) error {
	snapshot, err := controller.loadSharedSnapshot(ctx)
	if err != nil {
		return err
	}

	controller.applySnapshot(snapshot)

	return nil
}

func (controller *AdaptiveTTLController) applySnapshot(snapshot AdaptiveTTLSnapshot) {
	controller.mu.Lock()
	defer controller.mu.Unlock()

	controller.currentFactor = snapshot.Factor
	controller.currentState = snapshot.State

	controller.consecutiveStableReadings = snapshot.ConsecutiveStableReadings

	controller.consecutiveCongestedReadings = snapshot.ConsecutiveCongestedReadings
}
