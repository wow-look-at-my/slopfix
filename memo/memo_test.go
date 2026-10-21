package memo

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetComputesEachKeyOnce(t *testing.T) {
	c := New[string, int](8)
	calls := 0
	compute := func() int { calls++; return 7 }
	assert.Equal(t, 7, c.Get("a", compute))
	assert.Equal(t, 7, c.Get("a", compute))
	assert.Equal(t, 1, calls)
}

func TestAFullCacheStartsAgain(t *testing.T) {
	c := New[int, int](2)
	for i := range 5 {
		c.Get(i, func() int { return i })
	}
	assert.LessOrEqual(t, len(c.byKey), 2)
	assert.Equal(t, 4, c.Get(4, func() int { return -1 }))
}

func TestGetIsSafeAcrossGoroutines(t *testing.T) {
	c := New[int, int](16)
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			assert.Equal(t, i%8*2, c.Get(i%8, func() int { return i % 8 * 2 }))
		})
	}
	wg.Wait()
}
