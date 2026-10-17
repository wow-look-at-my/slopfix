// Package memo keeps the answers of pure functions for the rest of a run.
//
// A tree walk asks the same question of one file many times: each rule and
// each repair pass reads the same comments, blocks and sentences. A cache
// here answers the repeats from memory. Nothing goes to disk.
package memo

import "sync"

// Cache maps a key to the value a pure function computed for it. A full cache
// forgets every entry and fills again, so a long run stays bounded.
type Cache[K comparable, V any] struct {
	mu    sync.Mutex
	limit int
	byKey map[K]V
}

// New answers a cache that holds at most limit entries.
func New[K comparable, V any](limit int) *Cache[K, V] {
	return &Cache[K, V]{limit: limit, byKey: make(map[K]V)}
}

// Get answers the value for key. On a miss it calls compute and keeps the
// answer. compute runs outside the lock, so a slow answer blocks no other key.
// Two callers that miss on one key both compute it, and both get an equal value.
func (c *Cache[K, V]) Get(key K, compute func() V) V {
	c.mu.Lock()
	v, ok := c.byKey[key]
	c.mu.Unlock()
	if ok {
		return v
	}
	v = compute()
	c.mu.Lock()
	if len(c.byKey) >= c.limit {
		clear(c.byKey)
	}
	c.byKey[key] = v
	c.mu.Unlock()
	return v
}
