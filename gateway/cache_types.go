package main

import "time"

type KeyValue struct {
	Value      []byte
	Expiration time.Time
}

type CacheEntry struct {
	Key   string
	Value KeyValue
}
