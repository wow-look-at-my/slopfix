// Package timing records how long each phase of a run takes, and prints the
// breakdown when a run asks for it.
//
// A single elapsed figure says a run was slow. It does not say which rule, nor
// whether the cost was the rule at all rather than the parse underneath it.
// Every phase here is named for the thing a person would go and change, and the
// totals accumulate across every file a run reads, so a walk over a tree ends
// with one ranked list rather than a figure per file nobody can add up.
//
// Recording is off unless the run asks for it. An unrecorded phase costs a bool
// test and a call to a shared empty function.
package timing

import (
	"cmp"
	"fmt"
	"io"
	"strings"
	"time"
)

// EnvVar switches recording on from the environment, for a hook or a CI step
// that never reaches the command line.
const EnvVar = "SLOPFIX_TIMING"

// Line is one phase's totals across the run.
type Line struct {
	Name  string
	Calls int
	Total time.Duration
}

// noop closes a phase nobody is recording.
func noop() {}

// Track opens a named phase and returns the function that closes it, for a
// `defer timing.Track(name)()` at the top of the phase.
//
// Phases nest: a rule's span contains the parse the rule asks for, so the
// report names the rule and the substrate under it separately and a reader sees
// which of the two the time went to.
func Track(name string) func() {
	if !On() {
		return noop
	}
	start := time.Now()
	return func() { Record(name, time.Since(start)) }
}

// Report writes the breakdown, slowest phase first, and writes nothing at all
// when the run did not record.
//
// Wall clock is printed beside the phases. It is the denominator a reader
// needs, because phases nest and their totals do not add up to the run.
func Report(w io.Writer) {
	lines, wall := state.lines()
	if len(lines) == 0 {
		return
	}
	width := 0
	for _, l := range lines {
		width = max(width, len(l.Name))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\nslopfix timing -- wall %s\n", spell(wall))
	for _, l := range lines {
		fmt.Fprintf(&b, "  %-*s %10s  %5.1f%%  calls %-7d mean %s\n",
			width, l.Name, spell(l.Total), share(l.Total, wall), l.Calls, spell(l.Total/time.Duration(l.Calls)))
	}
	io.WriteString(w, b.String())
}

// share is a phase's part of the wall clock, and zero when no time passed.
func share(d, wall time.Duration) float64 {
	if wall <= 0 {
		return 0
	}
	return float64(d) / float64(wall) * 100
}

// spell cuts a duration to a width a person reads, keeping the unit its scale
// calls for.
func spell(d time.Duration) string {
	switch {
	case d >= time.Second:
		return d.Round(time.Millisecond).String()
	case d >= time.Millisecond:
		return d.Round(10 * time.Microsecond).String()
	default:
		return d.String()
	}
}

// byTotal orders the report: slowest first, and ties broken by name so two runs
// over the same tree print the same order.
func byTotal(a, b Line) int {
	if by := cmp.Compare(b.Total, a.Total); by != 0 {
		return by
	}
	return strings.Compare(a.Name, b.Name)
}
