package bashclean

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The stderr_merge rule exposes a grep the grep rule has already run past.
const interleaved = "cmd | grep x 2>&1"

func TestTheChainedRulesConverge(t *testing.T) {
	var warn strings.Builder
	got := transform(interleaved, "", maxPasses, &warn)
	assert.Equal(t, "set -o pipefail\ncmd\n", got.Command)
	assert.Empty(t, warn.String(), "a converged rewrite says nothing")
}

// A bound that runs out emits a HALF-APPLIED rewrite, which can be worse than
// either endpoint, and the caller is otherwise told it succeeded.
func TestExhaustingTheBoundSaysSo(t *testing.T) {
	var warn strings.Builder
	got := transform(interleaved, "", 1, &warn)

	assert.Contains(t, warn.String(), "did not reach a fixed point")
	assert.Contains(t, warn.String(), "undoing each other")
	assert.Contains(t, warn.String(), interleaved, "the message must name the command")
	assert.Contains(t, got.Command, "grep x", "the partial rewrite the message warns about")
}
