package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newL2TestHandler(t *testing.T) (*Handler, *miniredis.Miniredis) {
	t.Helper()

	redisServer := miniredis.RunT(t)

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisServer.Addr(),
	})

	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	handler := &Handler{
		redis: redisClient,
	}

	return handler, redisServer
}

func validL2Value(data string) KeyValue {
	return KeyValue{
		Value:             []byte(data),
		L2Expiration:      time.Now().Add(time.Minute),
		RecomputeDuration: 250 * time.Millisecond,
	}
}

func TestSetL2AndGetL2_ReturnsStoredValue(t *testing.T) {
	handler, _ := newL2TestHandler(t)

	ctx := context.Background()
	key := "/api/backend/1"
	expected := validL2Value("value-1")

	handler.setL2(ctx, key, expected, time.Minute, 10)

	actual, found := handler.getL2(ctx, key)

	if !found {
		t.Fatal("expected the value to be found in L2 cache")
	}

	if string(actual.Value) != string(expected.Value) {
		t.Fatalf("expected value %q, got %q", expected.Value, actual.Value)
	}

	if actual.RecomputeDuration != expected.RecomputeDuration {
		t.Fatalf("expected recompute duration %v, got %v", expected.RecomputeDuration, actual.RecomputeDuration)
	}

	if actual.L2Expiration.UnixMilli() != expected.L2Expiration.UnixMilli() {
		t.Fatalf("expected expiration %v, got %v", expected.L2Expiration, actual.L2Expiration)
	}
}

func TestGetL2_MissingKeyReturnsMiss(t *testing.T) {
	handler, _ := newL2TestHandler(t)

	ctx := context.Background()

	value, found := handler.getL2(ctx, "/api/backend/missing")

	if found {
		t.Fatal("expected an L2 cache miss")
	}

	if value.Value != nil {
		t.Fatal("expected an empty value for a cache miss")
	}
}

func TestSetL2_AppliesRedisTTL(t *testing.T) {
	handler, redisServer := newL2TestHandler(t)

	ctx := context.Background()
	key := "/api/backend/1"

	handler.setL2(ctx, key, validL2Value("value-1"), 5*time.Second, 10)

	if !redisServer.Exists(key) {
		t.Fatal("expected the value to exist before TTL expiration")
	}

	redisServer.FastForward(6 * time.Second)

	if redisServer.Exists(key) {
		t.Fatal("expected Redis to remove the value after TTL expiration")
	}

	_, found := handler.getL2(ctx, key)

	if found {
		t.Fatal("expected an L2 miss after TTL expiration")
	}

	_, err := handler.redis.ZScore(ctx, l2LRUKey, key).Result()

	if err != redis.Nil {
		t.Fatalf("expected the expired key to be removed from the LRU set, got error: %v", err)
	}
}

func TestGetL2_InvalidJSONIsRemoved(t *testing.T) {
	handler, redisServer := newL2TestHandler(t)

	ctx := context.Background()
	key := "/api/backend/1"

	err := handler.redis.Set(ctx, key, "invalid-json", time.Minute).Err()

	if err != nil {
		t.Fatalf("failed to prepare invalid Redis value: %v", err)
	}

	err = handler.redis.ZAdd(ctx, l2LRUKey,
		redis.Z{
			Score:  1,
			Member: key,
		},
	).Err()

	if err != nil {
		t.Fatalf("failed to prepare LRU entry: %v", err)
	}

	_, found := handler.getL2(ctx, key)

	if found {
		t.Fatal("expected invalid JSON to be treated as an L2 miss")
	}

	if redisServer.Exists(key) {
		t.Fatal("expected the invalid value to be removed from Redis")
	}

	_, err = handler.redis.ZScore(ctx, l2LRUKey, key).Result()

	if err != redis.Nil {
		t.Fatalf("expected the invalid key to be removed from the LRU set, got error: %v", err)
	}
}

func TestSetL2_EvictsLeastRecentlyUsedValue(t *testing.T) {
	handler, _ := newL2TestHandler(t)

	ctx := context.Background()

	handler.setL2(ctx, "/api/backend/1", validL2Value("value-1"), time.Minute, 2)

	time.Sleep(2 * time.Millisecond)

	handler.setL2(ctx, "/api/backend/2", validL2Value("value-2"), time.Minute, 2)

	time.Sleep(2 * time.Millisecond)

	if _, found := handler.getL2(ctx, "/api/backend/1"); !found {
		t.Fatal("expected the first key to exist")
	}

	time.Sleep(2 * time.Millisecond)

	handler.setL2(ctx, "/api/backend/3", validL2Value("value-3"), time.Minute, 2)

	if _, found := handler.getL2(ctx, "/api/backend/2"); found {
		t.Fatal("expected the least recently used key to be evicted")
	}

	if _, found := handler.getL2(ctx, "/api/backend/1"); !found {
		t.Fatal("expected the recently accessed key to remain")
	}

	if _, found := handler.getL2(ctx, "/api/backend/3"); !found {
		t.Fatal("expected the newly inserted key to remain")
	}
}

func TestSetL2_ZeroCapacityDoesNotStoreValue(t *testing.T) {
	handler, redisServer := newL2TestHandler(t)

	ctx := context.Background()
	key := "/api/backend/1"

	handler.setL2(ctx, key, validL2Value("value-1"), time.Minute, 0)

	if redisServer.Exists(key) {
		t.Fatal("expected no value to be stored when capacity is zero")
	}

	if redisServer.Exists(l2LRUKey) {
		t.Fatal("expected no LRU set when capacity is zero")
	}
}

func TestSetL2_CopiesInputData(t *testing.T) {
	handler, _ := newL2TestHandler(t)

	ctx := context.Background()
	key := "/api/backend/1"

	originalData := []byte("original")

	value := validL2Value("")
	value.Value = originalData

	handler.setL2(ctx, key, value, time.Minute, 10)

	originalData[0] = 'X'

	stored, found := handler.getL2(ctx, key)
	if !found {
		t.Fatal("expected the stored value to be found")
	}

	if string(stored.Value) != "original" {
		t.Fatalf("expected an independent copy, got %q", stored.Value)
	}
}

func TestClearL2_RemovesCachedValuesAndLRUSet(t *testing.T) {
	handler, redisServer := newL2TestHandler(t)

	ctx := context.Background()

	handler.setL2(ctx, "/api/backend/1", validL2Value("value-1"), time.Minute, 10)

	handler.setL2(ctx, "/api/backend/2", validL2Value("value-2"), time.Minute, 10)

	err := handler.clearL2(ctx)
	if err != nil {
		t.Fatalf("failed to clear L2 cache: %v", err)
	}

	if redisServer.Exists("/api/backend/1") {
		t.Fatal("expected the first cached value to be removed")
	}

	if redisServer.Exists("/api/backend/2") {
		t.Fatal("expected the second cached value to be removed")
	}

	if redisServer.Exists(l2LRUKey) {
		t.Fatal("expected the LRU sorted set to be removed")
	}
}
