package main

import (
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"
)

func shouldEarlyRefresh(value KeyValue, beta float64, now time.Time) (bool, float64, time.Duration) {

	if beta <= 0 {
		return false, 0, 0
	}

	if value.RecomputeDuration <= 0 {
		return false, 0, 0
	}

	if value.L2Expiration.IsZero() {
		return false, 0, 0
	}

	remaining := value.L2Expiration.Sub(now)
	if remaining <= 0 {
		return false, 0, remaining
	}

	randomValue := rand.Float64()

	if randomValue <= 0 {
		randomValue = math.SmallestNonzeroFloat64
	}

	randomGapSeconds := value.RecomputeDuration.Seconds() * beta * (-math.Log(randomValue))

	randomGap := time.Duration(randomGapSeconds * float64(time.Second))

	probability := math.Exp(-remaining.Seconds() / (value.RecomputeDuration.Seconds() * beta))

	return randomGap >= remaining, probability, remaining

}

func (h *Handler) tryProbabilisticEarlyRefresh(
	r *http.Request,
	key string,
	cachedValue KeyValue,
	ttlL2 time.Duration,
	config GatewayConfig,
) (KeyValue, bool) {
	now := time.Now()

	shouldRefresh, probability, remaining := shouldEarlyRefresh(cachedValue, config.EarlyRefreshBeta, now)

	if !shouldRefresh {
		return cachedValue, false
	}

	log.Printf(
		"Probabilistic early refresh selected: key=%s, remaining=%v, recomputeDuration=%v, beta=%.2f, probability=%.4f",
		key,
		remaining,
		cachedValue.RecomputeDuration,
		config.EarlyRefreshBeta,
		probability,
	)

	refreshedValue, statusCode, err := h.fetchFromBackend(r, config.BackendTimeoutMs)

	if err != nil {
		log.Printf(
			"Probabilistic early refresh failed for key %s: %v; serving cached value",
			key,
			err,
		)

		return cachedValue, false
	}

	if statusCode < 200 || statusCode >= 300 {
		log.Printf(
			"Probabilistic early refresh returned status %d for key %s; serving cached value",
			statusCode,
			key,
		)

		return cachedValue, false
	}

	refreshedValue.L2Expiration = cachedValue.L2Expiration.Add(ttlL2)

	redisTTL := time.Until(refreshedValue.L2Expiration)

	if redisTTL <= 0 {
		redisTTL = ttlL2

		refreshedValue.L2Expiration = time.Now().Add(ttlL2)
	}

	h.setL2(r.Context(), key, refreshedValue, redisTTL, config.L2MaxEntries)

	log.Printf(
		"Probabilistic early refresh completed: key=%s, oldExpiration=%s, newExpiration=%s, recomputeDuration=%v",
		key,
		cachedValue.L2Expiration.Format(
			time.RFC3339Nano,
		),
		refreshedValue.L2Expiration.Format(
			time.RFC3339Nano,
		),
		refreshedValue.RecomputeDuration,
	)

	return refreshedValue, true
}
