package main

import (
	"log"
	"time"
)

func (h *Handler) getL1(key string) (KeyValue, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	element, found := h.cache[key]
	if !found {
		return KeyValue{}, false
	}

	entry := element.Value.(*CacheEntry)

	if time.Now().After(entry.Value.Expiration) {
		delete(h.cache, key)
		h.cacheOrder.Remove(element)

		log.Println("L1 Expired: ", key)

		return KeyValue{}, false
	}
	h.cacheOrder.MoveToFront(element)

	return entry.Value, true
}

func (h *Handler) setL1(key string, value KeyValue) {
	h.mu.Lock()
	defer h.mu.Unlock()

	maxEntries := h.cacheConfig.L1MaxEntries

	if maxEntries <= 0 {
		return
	}

	if element, found := h.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		entry.Value = value

		h.cacheOrder.MoveToFront(element)
		return
	}

	if len(h.cache) >= maxEntries {
		h.removeL1LRU()
	}

	entry := &CacheEntry{
		Key:   key,
		Value: value,
	}

	element := h.cacheOrder.PushFront(entry)
	h.cache[key] = element

}

func (h *Handler) removeL1LRU() {
	element := h.cacheOrder.Back()
	if element == nil {
		return
	}

	entry := element.Value.(*CacheEntry)

	delete(h.cache, entry.Key)
	h.cacheOrder.Remove(element)

	log.Println("L1 EVICTED: ", entry.Key)
}
