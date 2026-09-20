package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/cardinal"
)

// Every entry in the table drives its own case. An entry that has stopped
// firing says so here rather than sitting in the file looking enforced.
func TestEveryTableEntryFires(t *testing.T) {
	require.NotEmpty(t, numbersTable.Rewrites)
	require.NotEmpty(t, numbersTable.Patterns)

	for _, r := range numbersTable.Rewrites {
		require.NotEmpty(t, r.Test, "rewrite %q carries no test", r.From)
		assert.Equal(t, r.Expect, Say(r.Test), "rewrite %q did not fire", r.From)
	}
	for _, p := range numbersTable.Patterns {
		require.NotEmpty(t, p.Test, "pattern %q carries no test", p.Match)
		assert.Equal(t, p.Expect, Say(p.Test), "pattern %q did not fire", p.Match)
	}
}

// What the table says leaves no number behind. An entry whose replacement
// carried another number would send the repair straight to a cut.
func TestWhatTheTableSaysCarriesNoNumber(t *testing.T) {
	for _, r := range numbersTable.Rewrites {
		assert.Empty(t, cardinal.Find(Say(r.Test), cardinal.Comment), "rewrite %q leaves a number", r.From)
	}
	for _, p := range numbersTable.Patterns {
		assert.Empty(t, cardinal.Find(Say(p.Test), cardinal.Comment), "pattern %q leaves a number", p.Match)
	}
}

// What the table writes for a whole line is stated in rules/numbers-cases.xml,
// where somebody adding a case edits no Go.
func TestEveryCaseHolds(t *testing.T) {
	require.NotEmpty(t, numbersTable.Cases)

	for _, c := range numbersTable.Cases {
		t.Run(c.Test, func(t *testing.T) {
			assert.Equal(t, c.Expect, Say(c.Test))
		})
	}
}
