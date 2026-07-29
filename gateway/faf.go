package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"
)

const (
	fafRefreshMarkerPrefix = "gateway:faf:refresh:"
	fafRefreshSafetyMargin = 2 * time.Second
)

func fafRefreshMarkerKey(cacheKey string) string {
	hash := sha256.Sum256([]byte(cacheKey))

	return fafRefreshMarkerPrefix + hex.EncodeToString(hash[:])
}

func (h *Handler) recordBackendCallForFAF(ctx context.Context, cacheKey string, backendTimeout time.Duration) {

	backendCallsByKey.WithLabelValues(cacheKey).Inc()

	markerKey := fafRefreshMarkerKey(cacheKey)

	markerTTL := backendTimeout + fafRefreshSafetyMargin

	created, err := h.redis.SetNX(ctx, markerKey, "1", markerTTL).Result()

	if err != nil {
		log.Printf("Failed to create FAF logical refresh marker for key %s: %v", cacheKey, err)
		return
	}

	if !created {
		//log.Printf("Backend call belongs to an existing logical refresh for key %s", cacheKey)
		return
	}

	logicalRefreshesByKey.WithLabelValues(cacheKey).Inc()

	//log.Printf("New logical refresh registered for key %s", cacheKey)

}
