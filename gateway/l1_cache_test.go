package main

import (
	"container/list"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newL1TestHandler() *Handler {
	return &Handler{
		cache:      make(map[string]*list.Element),
		cacheOrder: list.New(),
	}
}

func validL1Value(data string) KeyValue {
	return KeyValue{
		Value:        []byte(data),
		L1Expiration: time.Now().Add(time.Minute),
		L2Expiration: time.Now().Add(2 * time.Minute),
	}
}

func TestSetL1AndGetL1_ReturnsStoredValue(t *testing.T) {
	handler := newL1TestHandler()
	key := "/api/backend/1"

	handler.setL1(key, validL1Value("value-1"), 10)

	value, found := handler.getL1(key)

	if !found {
		t.Fatal("expected the value to be found in L1 cache")
	}

	if string(value.Value) != "value-1" {
		t.Fatalf("expected value %q, got %q", "value-1", value.Value)
	}

	if len(handler.cache) != 1 {
		t.Fatalf("expected one cache entry, got %d", len(handler.cache))
	}

	if handler.cacheOrder.Len() != 1 {
		t.Fatalf("expected one LRU list element, got %d", handler.cacheOrder.Len())
	}
}

func TestGetL1_MissingKeyReturnsMiss(t *testing.T) {
	handler := newL1TestHandler()

	value, found := handler.getL1("/api/backend/missing")

	if found {
		t.Fatal("expected a cache miss")
	}

	if value.Value != nil {
		t.Fatal("expected an empty value for a cache miss")
	}
}

func TestGetL1_RemovesValueWhenL1TTLExpires(t *testing.T) {
	handler := newL1TestHandler()
	key := "/api/backend/1"

	value := KeyValue{
		Value:        []byte("expired"),
		L1Expiration: time.Now().Add(-time.Second),
		L2Expiration: time.Now().Add(time.Minute),
	}

	handler.setL1(key, value, 10)

	_, found := handler.getL1(key)

	if found {
		t.Fatal("expected an L1 miss for an expired value")
	}

	if _, exists := handler.cache[key]; exists {
		t.Fatal("expected the expired value to be removed from the map")
	}

	if handler.cacheOrder.Len() != 0 {
		t.Fatal("expected the expired value to be removed from the LRU list")
	}
}

func TestGetL1_RemovesValueWhenL2TTLExpires(t *testing.T) {
	handler := newL1TestHandler()
	key := "/api/backend/1"

	value := KeyValue{
		Value:        []byte("expired"),
		L1Expiration: time.Now().Add(time.Minute),
		L2Expiration: time.Now().Add(-time.Second),
	}

	handler.setL1(key, value, 10)

	_, found := handler.getL1(key)

	if found {
		t.Fatal("expected an L1 miss when the corresponding L2 value expired")
	}

	if _, exists := handler.cache[key]; exists {
		t.Fatal("expected the value to be removed from the map")
	}

	if handler.cacheOrder.Len() != 0 {
		t.Fatal("expected the value to be removed from the LRU list")
	}
}

func TestGetL1_AllowsZeroL2Expiration(t *testing.T) {
	handler := newL1TestHandler()
	key := "/api/backend/1"

	value := KeyValue{
		Value:        []byte("value-1"),
		L1Expiration: time.Now().Add(time.Minute),
		L2Expiration: time.Time{},
	}

	handler.setL1(key, value, 10)

	_, found := handler.getL1(key)

	if !found {
		t.Fatal("expected zero L2 expiration not to invalidate the L1 value")
	}
}

func TestSetL1_EvictsLeastRecentlyUsedValue(t *testing.T) {
	handler := newL1TestHandler()

	handler.setL1("/api/backend/1", validL1Value("value-1"), 2)

	handler.setL1("/api/backend/2", validL1Value("value-2"), 2)

	_, found := handler.getL1("/api/backend/1")
	if !found {
		t.Fatal("expected the first key to exist")
	}

	handler.setL1("/api/backend/3", validL1Value("value-3"), 2)

	if _, found := handler.getL1("/api/backend/2"); found {
		t.Fatal("expected the least recently used key to be evicted")
	}

	if _, found := handler.getL1("/api/backend/1"); !found {
		t.Fatal("expected the recently accessed key to remain")
	}

	if _, found := handler.getL1("/api/backend/3"); !found {
		t.Fatal("expected the newly inserted key to remain")
	}

	if len(handler.cache) != 2 {
		t.Fatalf("expected cache size 2, got %d", len(handler.cache))
	}
}

func TestSetL1_UpdatesExistingValueAndLRUOrder(t *testing.T) {
	handler := newL1TestHandler()

	handler.setL1("/api/backend/1", validL1Value("old-value"), 2)

	handler.setL1("/api/backend/2", validL1Value("value-2"), 2)

	handler.setL1("/api/backend/1", validL1Value("new-value"), 2)

	handler.setL1("/api/backend/3", validL1Value("value-3"), 2)

	value, found := handler.getL1("/api/backend/1")
	if !found {
		t.Fatal("expected the updated key to remain in cache")
	}

	if string(value.Value) != "new-value" {
		t.Fatalf("expected updated value %q, got %q", "new-value", value.Value)
	}

	if _, found := handler.getL1("/api/backend/2"); found {
		t.Fatal("expected the second key to be evicted after updating the first")
	}
}

func TestSetL1_ZeroCapacityDoesNotStoreValue(t *testing.T) {
	handler := newL1TestHandler()

	handler.setL1("/api/backend/1", validL1Value("value-1"), 0)

	if len(handler.cache) != 0 {
		t.Fatal("expected no value to be stored when capacity is zero")
	}

	if handler.cacheOrder.Len() != 0 {
		t.Fatal("expected the LRU list to remain empty")
	}
}

func TestClearL1_RemovesAllValues(t *testing.T) {
	handler := newL1TestHandler()

	handler.setL1("/api/backend/1", validL1Value("value-1"), 10)

	handler.setL1("/api/backend/2", validL1Value("value-2"), 10)

	handler.clearL1()

	if len(handler.cache) != 0 {
		t.Fatalf("expected an empty cache, got %d entries", len(handler.cache))
	}

	if handler.cacheOrder.Len() != 0 {
		t.Fatalf("expected an empty LRU list, got %d elements", handler.cacheOrder.Len())
	}
}

func TestL1Cache_ConcurrentAccess(t *testing.T) {
	handler := newL1TestHandler()

	const requestCount = 100

	var waitGroup sync.WaitGroup
	waitGroup.Add(requestCount)

	for i := 0; i < requestCount; i++ {
		go func(index int) {
			defer waitGroup.Done()

			key := fmt.Sprintf("/api/backend/%d", index)

			handler.setL1(key, validL1Value(fmt.Sprintf("value-%d", index)), requestCount)

			if _, found := handler.getL1(key); !found {
				t.Errorf("expected key %q to be found", key)
			}
		}(i)
	}

	waitGroup.Wait()

	if len(handler.cache) != requestCount {
		t.Fatalf("expected %d cache entries, got %d", requestCount, len(handler.cache))
	}

	if handler.cacheOrder.Len() != requestCount {
		t.Fatalf("expected %d LRU elements, got %d", requestCount, handler.cacheOrder.Len())
	}
}
