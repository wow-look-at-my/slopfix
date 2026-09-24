package trace

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capture points the report at a buffer and turns tracing on for a single test.
func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	t.Setenv(EnvVar, "1")
	var buf bytes.Buffer
	mu.Lock()
	out, totals, counts = &buf, map[string]time.Duration{}, map[string]int{}
	mu.Unlock()
	return &buf
}

func TestAPhaseIsReportedWithItsName(t *testing.T) {
	buf := capture(t)

	Phase("walk")()
	Report()

	assert.Contains(t, buf.String(), "walk")
}

func TestRepeatedPhasesCarryTheirCount(t *testing.T) {
	buf := capture(t)

	for range 3 {
		Phase("measure")()
	}
	Report()

	assert.Contains(t, buf.String(), "x3")
}

// The slowest phase leads, which is the only ordering a reader wants.
func TestTheSlowestPhaseIsReportedFirst(t *testing.T) {
	t.Serial()
	buf := capture(t)

	mu.Lock()
	totals["fast"], counts["fast"] = time.Millisecond, 1
	totals["slow"], counts["slow"] = time.Second, 1
	mu.Unlock()
	Report()

	report := buf.String()
	require.Contains(t, report, "slow")
	assert.Less(t, indexOf(report, "slow"), indexOf(report, "fast"))
}

// Off is the default, and a phase then costs nothing and says nothing.
func TestTracingOffReportsNothing(t *testing.T) {
	t.Serial()
	t.Setenv(EnvVar, "")
	var buf bytes.Buffer
	mu.Lock()
	out = &buf
	mu.Unlock()

	Phase("walk")()
	Report()

	assert.Empty(t, buf.String())
}

func indexOf(haystack, needle string) int {
	return bytes.Index([]byte(haystack), []byte(needle))
}
