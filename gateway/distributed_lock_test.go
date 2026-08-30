package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newDistributedLockTestHandler(t *testing.T) (*Handler, *miniredis.Miniredis) {
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

func TestTryAcquireDistributedLock_FirstRequestAcquiresLock(t *testing.T) {
	handler, redisServer := newDistributedLockTestHandler(t)

	ctx := context.Background()
	lockKey := distributedLockKey("/api/backend/1")

	token, acquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !acquired {
		t.Fatal("expected the first request to acquire the lock")
	}

	if token == "" {
		t.Fatal("expected a non-empty ownership token")
	}

	storedToken, err := redisServer.Get(lockKey)
	if err != nil {
		t.Fatalf("expected the lock to exist in Redis: %v", err)
	}

	if storedToken != token {
		t.Fatalf("expected Redis to contain token %q, got %q", token, storedToken)
	}
}

func TestTryAcquireDistributedLock_OnlyOneRequestAcquiresLock(t *testing.T) {
	handler, _ := newDistributedLockTestHandler(t)

	ctx := context.Background()
	lockKey := distributedLockKey("/api/backend/1")

	firstToken, firstAcquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("first lock attempt failed: %v", err)
	}

	if !firstAcquired {
		t.Fatal("expected the first request to acquire the lock")
	}

	secondToken, secondAcquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("second lock attempt returned an error: %v", err)
	}

	if secondAcquired {
		t.Fatal("expected the second request not to acquire an existing lock")
	}

	if firstToken == secondToken {
		t.Fatal("expected different requests to generate different tokens")
	}
}

func TestReleaseDistributedLock_WrongTokenCannotReleaseLock(t *testing.T) {
	handler, redisServer := newDistributedLockTestHandler(t)

	ctx := context.Background()
	cacheKey := "/api/backend/1"
	lockKey := distributedLockKey(cacheKey)

	ownerToken, acquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("lock acquisition failed: %v", err)
	}

	if !acquired {
		t.Fatal("expected the lock to be acquired")
	}

	handler.releaseDistributedLock(lockKey, "incorrect-token", cacheKey)

	if !redisServer.Exists(lockKey) {
		t.Fatal("expected the lock to remain after release with a wrong token")
	}

	storedToken, err := redisServer.Get(lockKey)
	if err != nil {
		t.Fatalf("expected the lock to remain in Redis: %v", err)
	}

	if storedToken != ownerToken {
		t.Fatalf("expected the ownership token to remain unchanged")
	}
}

func TestReleaseDistributedLock_OwnerCanReleaseLock(t *testing.T) {
	handler, redisServer := newDistributedLockTestHandler(t)

	ctx := context.Background()
	cacheKey := "/api/backend/1"
	lockKey := distributedLockKey(cacheKey)

	token, acquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("lock acquisition failed: %v", err)
	}

	if !acquired {
		t.Fatal("expected the lock to be acquired")
	}

	handler.releaseDistributedLock(lockKey, token, cacheKey)

	if redisServer.Exists(lockKey) {
		t.Fatal("expected the owner to successfully release the lock")
	}
}

func TestDistributedLock_ExpiresAutomatically(t *testing.T) {
	handler, redisServer := newDistributedLockTestHandler(t)

	ctx := context.Background()
	lockKey := distributedLockKey("/api/backend/1")

	_, acquired, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("lock acquisition failed: %v", err)
	}

	if !acquired {
		t.Fatal("expected the lock to be acquired")
	}

	redisServer.FastForward(2 * time.Second)

	if redisServer.Exists(lockKey) {
		t.Fatal("expected the lock to expire automatically")
	}

	_, acquiredAgain, err := handler.tryAcquireDistributedLock(ctx, lockKey, time.Second)

	if err != nil {
		t.Fatalf("lock acquisition after expiration failed: %v", err)
	}

	if !acquiredAgain {
		t.Fatal("expected another request to acquire the expired lock")
	}
}
