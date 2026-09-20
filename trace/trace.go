// Package trace reports where slopfix spent its time.
//
// A hook runs on every tool call, so a phase that walks a tree costs the
// session that many traversals, and nothing in the output says so. Setting
// SLOPFIX_TRACE makes each phase report its own duration on stderr, which is
// where a hook's diagnostics go without reaching the model.
package trace

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// EnvVar turns tracing on when it holds anything other than the empty string.
const EnvVar = "SLOPFIX_TRACE"

var (
	mu     sync.Mutex
	totals = map[string]time.Duration{}
	counts = map[string]int{}

	// out is stderr, and a test points it elsewhere.
	out io.Writer = os.Stderr
)

// On reports whether tracing is enabled.
func On() bool { return os.Getenv(EnvVar) != "" }

// Phase times a span of work. The returned function ends it, so a caller
// writes `defer trace.Phase("walk")()` and spells the name a single time.
//
// It costs a clock read and a map write when tracing is off, so a phase can
// sit in a hot path.
func Phase(name string) func() {
	if !On() {
		return func() {}
	}
	start := time.Now()
	return func() { record(name, time.Since(start)) }
}

func record(name string, d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	totals[name] += d
	counts[name]++
}

// Report writes every phase seen, slowest and clears what it reported. A
// caller that never traced writes nothing.
func Report() {
	if !On() {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if len(totals) == 0 {
		return
	}

	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return totals[names[i]] > totals[names[j]] })

	fmt.Fprintln(out, "slopfix trace:")
	for _, name := range names {
		fmt.Fprintf(out, "  %-28s %8.1fms  x%d\n", name, float64(totals[name])/float64(time.Millisecond), counts[name])
	}

	totals = map[string]time.Duration{}
	counts = map[string]int{}
}
