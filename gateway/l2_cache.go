package main

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func (h *Handler) getL2(ctx context.Context, key string) ([]byte, bool) {
	value, err := h.redis.Get(ctx, key).Bytes()

	if err == redis.Nil {
		h.redis.ZRem(ctx, l2LRUKey, key)
		return nil, false
	}

	if err != nil {
		log.Println("Redis GET error: ", err)
		return nil, false
	}

	err = h.redis.ZAdd(ctx, l2LRUKey, redis.Z{
		Score:  float64(time.Now().UnixNano()),
		Member: key,
	}).Err()

	if err != nil {
		log.Println("Redis LRU update error: ", err)
	}

	return value, true
}

func (h *Handler) setL2(ctx context.Context, key string,
	value []byte, ttl time.Duration) {
	maxEntries := h.cacheConfig.L2MaxEntries

	if maxEntries <= 0 {
		return
	}

	err := h.redis.Set(ctx, key, value, ttl).Err()
	if err != nil {
		log.Println("Redis SET error: ", err)
		return
	}

	err = h.redis.ZAdd(ctx, l2LRUKey, redis.Z{
		Score:  float64(time.Now().UnixNano()),
		Member: key,
	}).Err()

	if err != nil {
		log.Println("Redis LRU ZADD error: ", err)
		return
	}

	count, err := h.redis.ZCard(ctx, l2LRUKey).Result()
	if err != nil {
		log.Println("Redis ZCARD error: ", err)
		return
	}

	if count > int64(maxEntries) {
		h.removeL2LRU(ctx, count-int64(maxEntries))
	}
}

func (h *Handler) removeL2LRU(ctx context.Context, count int64) {
	elements, err := h.redis.ZPopMin(ctx, l2LRUKey, count).Result()

	if err != nil {
		log.Println("Redis ZPOPMIN error: ", err)
		return
	}

	for _, element := range elements {
		key, ok := element.Member.(string)
		if !ok {
			continue
		}

		if err := h.redis.Del(ctx, key).Err(); err != nil {
			log.Println("Redis DEL error: ", err)
			continue
		}

		log.Println("L2 EVICTED: ", key)
	}
}
