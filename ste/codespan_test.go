package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// A division beside a code span ends the sentence after the closing backtick,
// never inside the span. The line is from a real CLAUDE.md.
func TestADivisionNeverLandsInsideACodeSpan(t *testing.T) {
	line := "- **This toolchain defaults to `GOOS=cosmo`.** Any `go build`/`go install`/`go test` run with the fork's `bin/go` targets cosmo unless you pin GOOS. Rebuilding a host tool needs e.g. `GOOS=linux GOARCH=amd64 go install cmd/link cmd/go`, and test harnesses (like `testdata/ape/apetest`) must be run with an upstream Go so the test binary itself is executable on the host."
	fixed := ste.Fix(line)
	assert.Equal(t, strings.Count(line, "`"), strings.Count(fixed, "`"), fixed)
	assert.Contains(t, fixed, "e.g. `GOOS=linux GOARCH=amd64 go install cmd/link cmd/go`")
	assert.NotContains(t, fixed, "cmd/go.", "a period landed inside the span")
}
