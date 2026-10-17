package gitread

import (
	"container/list"
	"sync"
)

// entryCost is the bookkeeping an entry costs beyond its bytes.
const entryCost = 96

// lru keeps values up to a byte budget, and drops the least recently used.
type lru[K comparable, V any] struct {
	mu     sync.Mutex
	budget int
	used   int
	order  list.List
	items  map[K]*list.Element
}

type lruEntry[K comparable, V any] struct {
	key  K
	val  V
	cost int
}

func newLRU[K comparable, V any](budget int) *lru[K, V] {
	return &lru[K, V]{budget: budget, items: map[K]*list.Element{}}
}

func (c *lru[K, V]) get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruEntry[K, V]).val, true
}

// put keeps val. A value too large for a share of the budget is not kept, so
// one large blob cannot empty the cache.
func (c *lru[K, V]) put(key K, val V, size int) {
	cost := size + entryCost
	if cost > c.budget/8 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry[K, V]{key: key, val: val, cost: cost})
	c.used += cost
	for c.used > c.budget {
		last := c.order.Back()
		entry := last.Value.(*lruEntry[K, V])
		c.order.Remove(last)
		delete(c.items, entry.key)
		c.used -= entry.cost
	}
}
