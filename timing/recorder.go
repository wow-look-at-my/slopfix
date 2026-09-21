package timing

import (
	"os"
	"slices"
	"sync"
	"time"
)

// recorder holds the running totals. A walk is serial today, and a lock keeps
// the answer right for the day it is not.
type recorder struct {
	mu      sync.Mutex
	on      bool
	from    time.Time
	totals  map[string]*Line
	arrived []string
}

// state is the process-wide recorder. The switch reads the environment once, so
// a hook and a CI step turn recording on without a command line.
var state = &recorder{
	on:     os.Getenv(EnvVar) != "",
	from:   time.Now(),
	totals: map[string]*Line{},
}

// On reports whether the run is recording.
func On() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.on
}

// Enable turns recording on for the rest of the process, and restarts the wall
// clock. A flag calls it before the first phase opens.
func Enable() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.on = true
	state.from = time.Now()
}

// Reset drops every recorded phase, so one test's spans do not reach another.
func Reset() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.totals = map[string]*Line{}
	state.arrived = nil
	state.from = time.Now()
}

// Record adds one span to a phase, for a caller holding a duration it measured
// some other way.
func Record(name string, d time.Duration) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.on {
		return
	}
	line, seen := state.totals[name]
	if !seen {
		line = &Line{Name: name}
		state.totals[name] = line
		state.arrived = append(state.arrived, name)
	}
	line.Calls++
	line.Total += d
}

// Lines answers every recorded phase, slowest first.
func Lines() []Line {
	lines, _ := state.lines()
	return lines
}

// lines copies the totals out under the lock, alongside the wall clock they
// are a part of.
func (r *recorder) lines() ([]Line, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.on {
		return nil, 0
	}
	out := make([]Line, 0, len(r.arrived))
	for _, name := range r.arrived {
		out = append(out, *r.totals[name])
	}
	slices.SortFunc(out, byTotal)
	return out, time.Since(r.from)
}
