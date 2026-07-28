package main

import "time"

type KeyValue struct {
	Value []byte

	L1Expiration time.Time
	L2Expiration time.Time

	RecomputeDuration time.Duration
}

type L2CacheValue struct {
	Value []byte `json:"value"`

	RecomputeDurationMs int64 `json:"recomputeDurationMs"`
	ExpiresAtUnixMs     int64 `json:"expiresAtUnixMs"`
}

type CacheEntry struct {
	Key   string
	Value KeyValue
}
