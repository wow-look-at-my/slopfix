package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/go-containers/set"
)

// A repair reflows a comment the fork wrote one line of. The fork's line
// takes its own repair, and the upstream line keeps every byte, so no word
// stands on both.
func TestAReflowAcrossAnUpstreamLineRepairsTheForksLine(t *testing.T) {
	before := "# rayon fan-out — both sit behind\n# non-default features, so it builds.\n"
	after := "# rayon fan-out. Both sit behind non-default\n# features, so it builds.\n"
	s := &Scope{lines: set.Of(1)}
	kept := Keep(before, after, s)
	lines := splitLines(kept)
	assert.Equal(t, "# rayon fan-out. Both sit behind\n", lines[0], "the fork's line is repaired")
	assert.Equal(t, "# non-default features, so it builds.\n", lines[1], "the upstream line is byte-identical")
	assert.Len(t, lines, 2)
}

// A rewrap that pulls the fork's first word onto the upstream line above it
// leaves that word on the fork's line. And still repairs the line.
func TestAReflowBelowAnUpstreamLineRepairsTheForksLine(t *testing.T) {
	before := "# the cache holds rows\n# and serves them — fast, warm and cheap.\n"
	after := "# the cache holds rows and\n# serves them. They are fast, warm and cheap.\n"
	s := &Scope{lines: set.Of(2)}
	lines := splitLines(Keep(before, after, s))
	assert.Equal(t, "# the cache holds rows\n", lines[0], "the upstream line is byte-identical")
	assert.Equal(t, "# and serves them. They are fast, warm and cheap.\n", lines[1], "the fork's line is repaired")
}

// A change confined to the fork's line lands.
func TestALineChangeOnTheForksLineLands(t *testing.T) {
	before := "# one, two\n# three four\n"
	after := "# one. Two\n# three four\n"
	s := &Scope{lines: set.Of(1)}
	assert.Equal(t, after, Keep(before, after, s))
}
