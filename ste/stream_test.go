package ste

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runOn is one sentence of about words words with no stop inside, as an API
// listing or a pasted log is.
func runOn(words int) string {
	clause := "the parser reads each line of the file and writes the result to the cache"
	n := len(strings.Fields(clause))
	parts := make([]string, 0, words/n+1)
	for range words/n + 1 {
		parts = append(parts, clause)
	}
	return "The " + strings.Join(parts, " ") + "."
}

func TestARunOnSentenceDividesToTheCap(t *testing.T) {
	out := fixSentenceCap(runOn(1200), capSpec{reorder: true, cap: SentenceWordCap, every: true})
	assert.Zero(t, overCap(out, SentenceWordCap), "every sentence of the repair fits the cap")
	assert.GreaterOrEqual(t, strings.Count(out, "parser"), strings.Count(runOn(1200), "parser"), "no word is lost")
}

// A division reads a bounded window, so four times the words take about four
// times as long. Rereading the whole rest at each division took sixteen.
func TestARunOnSentenceDividesInLinearTime(t *testing.T) {
	timed := func(words int) time.Duration {
		best := time.Duration(1 << 62)
		for range 2 {
			start := time.Now()
			fixSentenceCap(runOn(words), capSpec{reorder: true, cap: SentenceWordCap, every: true})
			best = min(best, time.Since(start))
		}
		return best
	}
	small, large := timed(1000), timed(4000)
	require.Positive(t, small)
	assert.Less(t, float64(large)/float64(small), 9.0, "small %v, large %v", small, large)
}
