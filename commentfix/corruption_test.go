package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// Whatever the repair writes, it writes words. A rewrite that runs of the
// author's words into a single says something the author did not, and nobody
// reads the diff a hook applied.
func TestTheRepairNeverWeldsTwoWordsTogether(t *testing.T) {
	for _, in := range []string{
		"Batch GET two of them.",
		"It fails when this one does not carry it.",
		"The walk is dropping the one top directory.",
		"It reads from the first .gitmodules parser above.",
		"It reads the first .gitmodules above.",
		"The tree keeps the first entry left.",
		"It holds one of the two .gitmodules files.",
	} {
		t.Run(in, func(t *testing.T) {
			assert.Empty(t, welded(in, Reword(in)))
		})
	}
}

// welded names an output word made of the input carried side by side, and is
// empty when the repair kept them apart.
func welded(in, out string) string {
	have := set.Of(strings.Fields(in)...)
	for _, word := range strings.Fields(out) {
		if have.Contains(word) {
			continue
		}
		for cut := 1; cut < len(word); cut++ {
			if have.Contains(word[:cut]) && have.Contains(word[cut:]) {
				return word
			}
		}
	}
	return ""
}

// The repair leaves a number behind for the sentence cut, and the cut is what
// clears the finding. Neither may leave the line saying nothing at all.
func TestTheRepairLeavesNoNumberStanding(t *testing.T) {
	src := "package p\n\n// It holds the two .gitmodules files.\nvar x int\n"
	out := Fix("x.go", src)
	require.True(t, out.Changed)
	assert.Empty(t, Check("x.go", out.Text))
}
