package slopfix

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressEvery is how often a tree run says where it is.
const ProgressEvery = 5 * time.Second

var progress struct {
	mu    sync.Mutex
	phase string
	done  atomic.Int64
	total atomic.Int64
}

// SetPhase names the step a tree run is in. A total above zero counts the
// files the step works through.
func SetPhase(name string, total int) {
	progress.mu.Lock()
	progress.phase = name
	progress.mu.Unlock()
	progress.done.Store(0)
	progress.total.Store(int64(total))
}

// stepDone counts one file of the current step.
func stepDone() { progress.done.Add(1) }

// ReportProgress writes the current step to w every interval until stop is called.
func ReportProgress(w io.Writer, every time.Duration) (stop func()) {
	start := time.Now()
	quit := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tick.C:
				fmt.Fprintln(w, progressLine(time.Since(start)))
			}
		}
	})
	return func() {
		close(quit)
		wg.Wait()
	}
}

// progressLine answers the line ReportProgress writes after elapsed.
func progressLine(elapsed time.Duration) string {
	progress.mu.Lock()
	phase := progress.phase
	progress.mu.Unlock()
	if phase == "" {
		phase = "start"
	}
	line := fmt.Sprintf("slopfix: %s", phase)
	if total := progress.total.Load(); total > 0 {
		line += fmt.Sprintf(" %d/%d", progress.done.Load(), total)
	}
	return line + fmt.Sprintf(" (%s)", elapsed.Truncate(time.Second))
}
