package bashclean

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The trailing rules strip a grep and a merge from the final statement.
const interleaved = "cmd | grep x 2>&1"

func TestTheChainedRulesConverge(t *testing.T) {
	var warn strings.Builder
	got := transform(interleaved, "", maxPasses, &warn)
	assert.Equal(t, "set -o pipefail\ncmd\n", got.Command)
	assert.Empty(t, warn.String(), "a converged rewrite says nothing")
}

// A bound that runs out may emit a half-applied rewrite, so it must say so.
func TestExhaustingTheBoundSaysSo(t *testing.T) {
	var warn strings.Builder
	transform(interleaved, "", 1, &warn)

	assert.Contains(t, warn.String(), "did not reach a fixed point")
	assert.Contains(t, warn.String(), "undoing each other")
	assert.Contains(t, warn.String(), interleaved, "the message must name the command")
}
