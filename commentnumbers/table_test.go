package commentnumbers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/cardinal"
)

// Every entry in the table drives its own case. An entry that has stopped
// firing says so here rather than sitting in the file looking enforced.
func TestEveryTableEntryFires(t *testing.T) {
	require.NotEmpty(t, table.Rewrites)
	require.NotEmpty(t, table.Patterns)

	for _, r := range table.Rewrites {
		require.NotEmpty(t, r.Test, "rewrite %q carries no test", r.From)
		assert.Equal(t, r.Expect, Say(r.Test), "rewrite %q did not fire", r.From)
	}
	for _, p := range table.Patterns {
		require.NotEmpty(t, p.Test, "pattern %q carries no test", p.Match)
		assert.Equal(t, p.Expect, Say(p.Test), "pattern %q did not fire", p.Match)
	}
}

// What the table says leaves no number behind. An entry whose replacement
// carried another number would send the repair straight to a cut.
func TestWhatTheTableSaysCarriesNoNumber(t *testing.T) {
	for _, r := range table.Rewrites {
		assert.Empty(t, cardinal.Find(Say(r.Test), cardinal.Comment), "rewrite %q leaves a number", r.From)
	}
	for _, p := range table.Patterns {
		assert.Empty(t, cardinal.Find(Say(p.Test), cardinal.Comment), "pattern %q leaves a number", p.Match)
	}
}

// The negative control. Prose the table says nothing about comes back as it went
// in, so the cases above pass on a rewrite rather than on any edit at all.
func TestProseTheTableDoesNotCoverIsUntouched(t *testing.T) {
	assert.Equal(t, "It reserves a slot and publishes it", Say("It reserves a slot and publishes it"))
}

// A comment line often continues a wrapped sentence rather than opening it, so
// the repair keeps the opening case the author wrote.
func TestTheOpeningCaseIsTheOneTheAuthorWrote(t *testing.T) {
	assert.Equal(t, "a single side of s or other.", Say("exactly one of s or other."))
	assert.Equal(t, "Goroutines contend", Say("Two goroutines contend"))
}

// A word that merely contains a number word is a name, so the table's own
// matching leaves it alone.
func TestTheTableLeavesAWordContainingANumberWordAlone(t *testing.T) {
	assert.Equal(t, "The oneShot flag and someone else", Say("The oneShot flag and someone else"))
}
