package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// The sentences a repair cut into fragments, verbatim from the file it wrote
// them into. A long sentence is a finding. A sentence with no subject is a
// defect the reader cannot repair from what is left.
var fragmented = []string{
	"`bin` is what keeps that acyclic: the helpers, the backends and the daemon all name an `Item` without naming each other.",
	"Every backend's `resolveID` checks that the ID names an existing entry inside one of *this user's* trash directories before restoring anything.",
	"That implementation is the `ds_store` Python package behind `dmgbuild`, which is what keeps the codec honest.",
	"`finder_trash_many.DS_Store` is a tree with internal nodes carrying the records this package wrote.",
	"A rename into the trash is a different operation: the kernel refuses one whose source the caller may not rename out of even where it may unlink it.",
}

func TestARepairNeverWritesAFragment(t *testing.T) {
	for _, prose := range fragmented {
		out := ste.Fix(prose)
		for _, sentence := range ste.Sentences(out) {
			words := strings.Fields(sentence)
			assert.NotEmpty(t, words, "empty sentence from %q", prose)
			assert.NotEqual(t, ".", strings.TrimSpace(sentence),
				"bare stop from %q", prose)
		}
		assert.NotContains(t, out, " an. ", "cut after an article: %q", out)
		assert.NotContains(t, out, " the. ", "cut after an article: %q", out)
		assert.NotContains(t, out, " is. ", "cut after a verb: %q", out)
	}
}

// A sentence offering no clause seam keeps its length. The finding still names
// it, so nothing is hidden - the repair declines rather than inventing a break.
func TestAnUnsplittableSentenceIsLeftWhole(t *testing.T) {
	prose := "Every backend's `resolveID` checks that the ID names an existing entry inside one of *this user's* trash directories before restoring anything."

	assert.Equal(t, prose, ste.Fix(prose))
	assert.NotEmpty(t, ste.Check(prose, 1), "the length is still reported")
}

// The control: a real clause join still divides, so the repair has not simply
// been turned off.
func TestAClauseJoinStillDivides(t *testing.T) {
	prose := "The daemon reads every filesystem that holds a bin and reports the pressure it finds there, and the sweep then gives back the oldest items it can account for."

	out := ste.Fix(prose)
	assert.NotEqual(t, prose, out)
	assert.Greater(t, len(ste.Sentences(out)), 1)
}
