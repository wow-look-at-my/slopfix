// Package timing records how long each phase of a run takes, and prints the
// breakdown when a run asks for it.
//
// A single elapsed figure says a run was slow. It does not say which rule, or
// whether the cost was the rule at all rather than the parse underneath it.
// Every phase here is named for the thing a person would go and change, and the
// totals accumulate across every file a run reads, so a walk over a tree ends
// with one ranked list rather than a number per file nobody can add up.
//
// Recording is off unless the run asks. A phase that is not recorded costs a
// bool test and a call to a shared empty function.
package timing

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// EnvVar switches recording on from the environment, for a hook or a CI step
// that never reaches the command line.
const EnvVar = "SLOPFIX_TIMING"

var (
	mu       sync.Mutex
	enabled  = os.Getenv(EnvVar) != ""
	phases   = map[string]*phase{}
	order    []string
	noop     = func() {}
	wallFrom = time.Now()
)

// phase is one named span's running totals.
type phase struct {
	calls int
	total time.Duration
}

// Enable turns recording on for the rest of the process. A flag calls this
// before any phase runs.
func Enable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = true
	wallFrom = time.Now()
}

// On reports whether the run is recording.
func On() bool {
	mu.Lock()
	defer mu.Unlock()
	return enabled
}

// Track opens a named phase and returns the function that closes it, for a
// `defer timing.Track(name)()` at the top of the phase.
//
// Phases nest: a rule's span contains the parse it asks for, so the report
// names a rule and the substrate under it separately and a reader sees which
// of the two the time went to.
func Track(name string) func() {
	if !On() {
		return noop
	}
	start := time.Now()
	return func() { Record(name, time.Since(start)) }
}

// Record adds one span to a phase, for a caller that already holds the
// duration.
func Record(name string, d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	if !enabled {
		return
	}
	p, seen := phases[name]
	if !seen {
		p = &phase{}
		phases[name] = p
		order = append(order, name)
	}
	p.calls++
	p.total += d
}

// Reset drops every recorded phase, so one test's spans do not reach another.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	phases = map[string]*phase{}
	order = nil
	wallFrom = time.Now()
}

// line is one phase, ready to print.
type line struct {
	name  string
	calls int
	total time.Duration
}

// Lines answers every recorded phase, slowest first, and ties broken by name so
// two runs of the same tree print the same order.
func Lines() []struct {
	Name  string
	Calls int
	Total time.Duration
} {
	mu.Lock()
	defer mu.Unlock()
	out := make([]line, 0, len(order))
	for _, name := range order {
		p := phases[name]
		out = append(out, line{name: name, calls: p.calls, total: p.total})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].total != out[j].total {
			return out[i].total > out[j].total
		}
		return out[i].name < out[j].name
	})
	wire := make([]struct {
		Name  string
		Calls int
		Total time.Duration
	}, len(out))
	for i, l := range out {
		wire[i] = struct {
			Name  string
			Calls int
			Total time.Duration
		}{Name: l.name, Calls: l.calls, Total: l.total}
	}
	return wire
}

// Report writes the breakdown, slowest phase first, and writes nothing at all
// when the run did not record.
//
// Wall clock is printed beside the phases. It is the denominator a reader
// needs: phases nest and overlap, so their totals do not add up to the run.
func Report(w io.Writer) {
	if !On() {
		return
	}
	lines := Lines()
	if len(lines) == 0 {
		return
	}
	mu.Lock()
	wall := time.Since(wallFrom)
	mu.Unlock()
	fmt.Fprintf(w, "\nslopfix timing -- wall %s\n", round(wall))
	width := 0
	for _, l := range lines {
		if len(l.Name) > width {
			width = len(l.Name)
		}
	}
	for _, l := range lines {
		mean := l.Total / time.Duration(l.calls())
		fmt.Fprintf(w, "  %-*s %9s  %6.1f%%  calls %-7d mean %s\n",
			width, l.Name, round(l.Total), percent(l.Total, wall), l.Calls, round(mean))
	}
}

// percent is a phase's share of the wall clock, and zero when no time passed.
func percent(d, wall time.Duration) float64 {
	if wall <= 0 {
		return 0
	}
	return float64(d) / float64(wall) * 100
}

// round cuts a duration to a width a person reads, keeping the unit the scale
// calls for.
func round(d time.Duration) string {
	switch {
	case d >= time.Second:
		return d.Round(time.Millisecond).String()
	case d >= time.Millisecond:
		return d.Round(10 * time.Microsecond).String()
	default:
		return d.Round(time.Nanosecond).String()
	}
}
