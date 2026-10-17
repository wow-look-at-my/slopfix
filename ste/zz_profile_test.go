package ste

import (
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZZProfileRunOn(t *testing.T) {
	t.Serial()
	path := os.Getenv("SLOPFIX_RUNON_PROFILE")
	if path == "" {
		t.Skip("no profile path")
	}
	counts := map[string]int{}
	sizes := map[string]int{}
	probeForce = func(event string, n int) { counts[event]++; sizes[event] = max(sizes[event], n) }
	fixSentenceCap(runOn(2000), capSpec{reorder: true, cap: SentenceWordCap, every: true})
	t.Logf("events %v max sizes %v", counts, sizes)
	probeForce = func(string, int) {}
	for _, n := range []int{500, 1000, 2000, 4000} {
		start := time.Now()
		fixSentenceCap(runOn(n), capSpec{reorder: true, cap: SentenceWordCap, every: true})
		t.Logf("words %d: %v", n, time.Since(start))
	}
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, pprof.StartCPUProfile(f))
	fixSentenceCap(runOn(4000), capSpec{reorder: true, cap: SentenceWordCap, every: true})
	pprof.StopCPUProfile()
}
