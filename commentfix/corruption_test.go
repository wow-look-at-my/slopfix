package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The strings here were taken out of repositories the number repair had already
// been run over. Each input is a comment as its author wrote it, and each want
// is what the repair has to write instead of the wreckage it shipped.
//
// A repair nobody reviews is a repair that has to be right, so these are driven
// rather than described.
func TestTheRepairWritesEnglishOverTheNumber(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
	}{
		// It deleted the cardinal and trusted "of" to carry the meaning, which
		// left a sentence with no object in it: "Batch GET of them."
		{
			name: "a cardinal governing no noun",
			in:   "Batch GET two of them.",
			want: "Batch GET some of them.",
		},
		{
			name: "one standing in for a noun",
			in:   "It fails when this one does not carry it.",
			want: "It fails when this does not carry it.",
		},
		// A determiner is already there, so another a single wrote "the a single".
		{
			name: "one behind a determiner",
			in:   "The walk is dropping the one top directory.",
			want: "The walk is dropping the top directory.",
		},
		// The tally still applies where the cardinal governs a plural noun.
		{
			name: "a cardinal governing a plural noun",
			in:   "Two goroutines contend for it.",
			want: "Goroutines contend for it.",
		},
		// A single counting a thing is a count, and "a single" says it in
		// words. The reading is awkward and every word survives it.
		{
			name: "one counting a thing",
			in:   "One good entry and one relic.",
			want: "A single good entry and a single relic.",
		},
		// A repeat count. The same awkward reading, and nothing lost.
		{
			name: "a repeat count",
			in:   "It is a no-op once.",
			want: "It is a no-op a single time.",
		},
		// An ordinal naming a position. A file name beside it keeps its dot
		// AND the space in front of the dot.
		{
			name: "an ordinal beside a file name",
			in:   "It reads from the first .gitmodules parser above.",
			want: "It reads from the earliest .gitmodules parser above.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Reword(c.in))
		})
	}
}

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
	have := make(map[string]bool)
	for _, word := range strings.Fields(in) {
		have[word] = true
	}
	for _, word := range strings.Fields(out) {
		if have[word] {
			continue
		}
		for cut := 1; cut < len(word); cut++ {
			if have[word[:cut]] && have[word[cut:]] {
				return word
			}
		}
	}
	return ""
}

// The repair leaves a number behind for the sentence cut, and the cut is what
// clears the finding. Neither may leave the line saying nothing at all.
func TestTheRepairLeavesNoNumberStanding(t *testing.T) {
	src := "package p\n\n// It holds the one .gitmodules file.\nvar x int\n"
	out := Fix("x.go", src)
	require.True(t, out.Changed)
	assert.Empty(t, Check("x.go", out.Text))
}
