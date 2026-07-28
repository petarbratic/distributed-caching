package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	distributedLockPrefix = "gateway:lock:"
	lockPollInterval      = 25 * time.Millisecond
	lockSafetyMargin      = 2 * time.Millisecond
)

var releaseDistributedLockScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end

	return 0
`)

func distributedLockKey(cacheKey string) string {
	hash := sha256.Sum256([]byte(cacheKey))

	return distributedLockPrefix + hex.EncodeToString(hash[:])
}

func newDistributedLockToken() (string, error) {
	tokenBytes := make([]byte, 16)

	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate distributed lock token: %w", err)
	}

	return hex.EncodeToString(tokenBytes), nil
}

func (h *Handler) tryAcquireDistributedLock(
	ctx context.Context,
	lockKey string,
	ttl time.Duration,
) (string, bool, error) {
	token, err := newDistributedLockToken()
	if err != nil {
		return "", false, err
	}

	acquired, err := h.redis.SetNX(
		ctx,
		lockKey,
		token,
		ttl,
	).Result()
	if err != nil {
		return "", false, fmt.Errorf(
			"acquire distributed lock: %w",
			err,
		)
	}

	return token, acquired, nil
}

func (h *Handler) releaseDistributedLock(
	lockKey string,
	token string,
	cacheKey string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	released, err := releaseDistributedLockScript.Run(
		ctx,
		h.redis,
		[]string{lockKey},
		token,
	).Int64()

	if err != nil {
		log.Printf("Distributed lock release failed for key %s: %v",
			cacheKey,
			err,
		)
		return
	}

	if released == 0 {
		log.Printf(
			"Distributed lock for key %s was not released because ownership changed or the lock expired",
			cacheKey,
		)
		return
	}

	log.Printf(
		"Distributed lock released for key %s",
		cacheKey,
	)
}

func (h *Handler) fetchWithDistributedLock(
	r *http.Request,
	cacheKey string,
	ttlL2 time.Duration,
	config GatewayConfig,
) (KeyValue, int, error) {
	lockKey := distributedLockKey(cacheKey)

	lockTTL := time.Duration(config.BackendTimeoutMs)*time.Millisecond + lockSafetyMargin

	waitTimeout := lockTTL + lockSafetyMargin

	waitContext, cancel := context.WithTimeout(r.Context(), waitTimeout)
	defer cancel()

	waitingLogged := false

	for {
		token, acquired, err := h.tryAcquireDistributedLock(waitContext, lockKey, lockTTL)
		if err != nil {
			log.Printf("Distributed lock attempt failed for key %s: %v", cacheKey, err)
			return KeyValue{}, http.StatusServiceUnavailable, err
		}

		if acquired {
			log.Printf("Distributed lock acquired for key %s", cacheKey)
			return h.fetchAsDistributedLockOwner(r, cacheKey, lockKey, token, ttlL2, config)
		}

		if !waitingLogged {
			log.Printf("Distributed lock busy for key %s; waiting for L2 value", cacheKey)
			waitingLogged = true
		}

		value, found := h.getL2(waitContext, cacheKey)
		if found {
			log.Printf("L2 value became available while waiting for distributed lock for key %s", cacheKey)
			return value, http.StatusOK, nil
		}

		lockExists, err := h.redis.Exists(waitContext, lockKey).Result()
		if err != nil {
			return KeyValue{}, http.StatusServiceUnavailable, fmt.Errorf(
				"check distributed lock: %w", err,
			)
		}

		if lockExists == 0 {
			log.Printf("Distributed lock disappeared without an L2 value for key %s; retrying", cacheKey)
			continue
		}

		select {
		case <-time.After(lockPollInterval):
		case <-waitContext.Done():
			log.Printf("Timed out while waiting for distributed lock for key %s", cacheKey)
			return KeyValue{}, http.StatusGatewayTimeout, waitContext.Err()
		}
	}
}

func (h *Handler) fetchAsDistributedLockOwner(
	r *http.Request,
	cacheKey string,
	lockKey string,
	token string,
	ttlL2 time.Duration,
	config GatewayConfig,
) (KeyValue, int, error) {
	defer h.releaseDistributedLock(lockKey, token, cacheKey)

	value, found := h.getL2(r.Context(), cacheKey)
	if found {
		log.Printf("L2 value already exists after acquiring distributed lock for key %s; backend call skipped", cacheKey)
		return value, http.StatusOK, nil
	}

	log.Printf("Distributed lock owner is calling backend for key %s", cacheKey)

	value, statusCode, err := h.fetchFromBackend(r, config.BackendTimeoutMs)

	if err != nil {
		log.Printf("Backend call failed while holding distributed lock for key %s: %v", cacheKey, err)
		return value, statusCode, err
	}

	if statusCode >= 200 && statusCode < 300 {
		value.L2Expiration = time.Now().Add(ttlL2)

		h.setL2(r.Context(), cacheKey, value, ttlL2, config.L2MaxEntries)
		log.Printf("Distributed lock owner stored backend response in L2 for key %s", cacheKey)
	}

	return value, statusCode, nil

}
