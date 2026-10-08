package slopfix

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestTheProgressLineNamesThePhaseAndTheCount(t *testing.T) {
	SetPhase("judge the files", 4)
	stepDone()
	assert.Equal(t, "slopfix: judge the files 1/4 (7s)", progressLine(7*time.Second+300*time.Millisecond))
	SetPhase("index the names", 0)
	assert.Equal(t, "slopfix: index the names (0s)", progressLine(0))
}

// The report runs on its own clock, so a step that never returns still prints.
func TestTheReportPrintsOnEveryTickUntilStopped(t *testing.T) {
	SetPhase("read the fork scope", 0)
	var out lockedBuffer
	stop := ReportProgress(&out, 10*time.Millisecond)
	time.Sleep(55 * time.Millisecond)
	stop()
	lines := strings.Count(out.String(), "slopfix: read the fork scope")
	assert.GreaterOrEqual(t, lines, 3)
	printed := out.String()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, printed, out.String(), "nothing prints after stop")
}
