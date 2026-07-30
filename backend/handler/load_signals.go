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

const loadSignalWindowSize = 200

type LoadSignals struct {
	P99LatencyMs          float64 `json:"p99LatencyMs"`
	ActiveRequests        int64   `json:"activeRequests"`
	MaxConcurrentRequests int     `json:"maxConcurrentRequests"`
	Utilization           float64 `json:"ulization"`
	SampleCount           int     `json:"sampleCount"`
}

func (h *Handler) recordRequestDuration(duration time.Duration) {
	h.loadSignalsMu.Lock()
	defer h.loadSignalsMu.Unlock()

	if len(h.requestDurations) >= loadSignalWindowSize {
		copy(h.requestDurations, h.requestDurations[1:])
		h.requestDurations[loadSignalWindowSize-1] = duration
		return
	}

	h.requestDurations = append(h.requestDurations, duration)
}

func (h *Handler) calculateP99LatencyMs() float64 {
	h.loadSignalsMu.RLock()

	durations := make([]time.Duration, len(h.requestDurations))

	copy(durations, h.requestDurations)

	h.loadSignalsMu.RUnlock()

	if len(durations) == 0 {
		return 0
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	index := int(math.Ceil(0.99*float64(len(durations)))) - 1

	return float64(durations[index]) / float64(time.Millisecond)
}

func (h *Handler) GetLoadSignals(w http.ResponseWriter, req *http.Request) {
	h.configMu.RLock()

	maxConcurrentRequests := cap(h.Semaphore)
	activeRequests := atomic.LoadInt64(&h.Active)

	h.configMu.RUnlock()

	utilization := 0.0

	if maxConcurrentRequests > 0 {
		utilization = float64(activeRequests) / float64(maxConcurrentRequests)
	}

	h.loadSignalsMu.RLock()
	sampleCount := len(h.requestDurations)
	h.loadSignalsMu.RUnlock()

	signals := LoadSignals{
		P99LatencyMs:          h.calculateP99LatencyMs(),
		ActiveRequests:        activeRequests,
		MaxConcurrentRequests: maxConcurrentRequests,
		Utilization:           utilization,
		SampleCount:           sampleCount,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(signals); err != nil {
		log.Println("Failed to encode backend load signals: ", err)
	}
}
