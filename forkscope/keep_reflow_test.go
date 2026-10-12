package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/go-containers/set"
)

// sglang .pre-commit-config.yaml: a repair reflows a comment the fork wrote
// one line of. Keeping the fork's new line beside the upstream line it
// replaced wrote "non-default" twice, so the reflow goes back whole.
func TestAReflowAcrossAnUpstreamLineGoesBackWhole(t *testing.T) {
	before := "# rayon fan-out — both sit behind\n# non-default features, so it builds.\n"
	after := "# rayon fan-out. Both sit behind non-default\n# features, so it builds.\n"
	s := &Scope{lines: set.Of(1)}
	assert.Equal(t, before, Keep(before, after, s))
}

// A change confined to the fork's line lands.
func TestALineChangeOnTheForksLineLands(t *testing.T) {
	before := "# one, two\n# three four\n"
	after := "# one. Two\n# three four\n"
	s := &Scope{lines: set.Of(1)}
	assert.Equal(t, after, Keep(before, after, s))
}
