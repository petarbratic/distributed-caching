package handler

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"sync/atomic"
	"time"
)

const loadSignalWindowDuration = 15 * time.Second

type latencySample struct {
	RecordedAt time.Time
	Duration   time.Duration
}

type LoadSignals struct {
	P99LatencyMs          float64 `json:"p99LatencyMs"`
	ActiveRequests        int64   `json:"activeRequests"`
	MaxConcurrentRequests int     `json:"maxConcurrentRequests"`
	Utilization           float64 `json:"utilization"`
	SampleCount           int     `json:"sampleCount"`
}

func (h *Handler) recordRequestDuration(
	duration time.Duration,
) {
	now := time.Now()

	h.loadSignalsMu.Lock()
	defer h.loadSignalsMu.Unlock()

	h.requestDurations = append(
		h.requestDurations,
		latencySample{
			RecordedAt: now,
			Duration:   duration,
		},
	)

	h.removeExpiredLatencySamplesLocked(now)
}

func (h *Handler) removeExpiredLatencySamplesLocked(now time.Time) {
	cutoff := now.Add(-loadSignalWindowDuration)

	firstRecentIndex := sort.Search(
		len(h.requestDurations),
		func(index int) bool {
			return !h.requestDurations[index].RecordedAt.Before(cutoff)
		},
	)

	if firstRecentIndex == 0 {
		return
	}

	if firstRecentIndex >= len(h.requestDurations) {
		h.requestDurations = nil
		return
	}

	recentSamples := append(
		[]latencySample(nil),
		h.requestDurations[firstRecentIndex:]...,
	)

	h.requestDurations = recentSamples
}

func (h *Handler) recentRequestDurations(now time.Time) []time.Duration {
	h.loadSignalsMu.Lock()
	defer h.loadSignalsMu.Unlock()

	h.removeExpiredLatencySamplesLocked(now)

	durations := make([]time.Duration, len(h.requestDurations))

	for index, sample := range h.requestDurations {
		durations[index] = sample.Duration
	}

	return durations
}

func calculateP99LatencyMs(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	sortedDurations := append(
		[]time.Duration(nil),
		durations...,
	)

	sort.Slice(
		sortedDurations,
		func(first int, second int) bool {
			return sortedDurations[first] <
				sortedDurations[second]
		},
	)

	index := int(math.Ceil(0.99*float64(len(sortedDurations)))) - 1

	if index < 0 {
		index = 0
	}

	return float64(sortedDurations[index]) / float64(time.Millisecond)
}

func (h *Handler) GetLoadSignals(w http.ResponseWriter, req *http.Request) {
	h.configMu.RLock()

	maxConcurrentRequests := cap(h.Semaphore)
	activeRequests := atomic.LoadInt64(&h.Active)

	h.configMu.RUnlock()

	utilization := 0.0

	if maxConcurrentRequests > 0 {
		utilization =
			float64(activeRequests) /
				float64(maxConcurrentRequests)
	}

	durations := h.recentRequestDurations(time.Now())

	signals := LoadSignals{
		P99LatencyMs: calculateP99LatencyMs(
			durations,
		),
		ActiveRequests:        activeRequests,
		MaxConcurrentRequests: maxConcurrentRequests,
		Utilization:           utilization,
		SampleCount:           len(durations),
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(signals); err != nil {
		log.Println("Failed to encode backend load signals: ", err)
	}
}
