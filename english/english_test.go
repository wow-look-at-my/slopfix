package english

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every entry declares the test that proves it fires. The firing itself is
// asserted where the repair lives, because an entry only means anything once a
// repair has applied it.
func TestEveryEntryDeclaresItsOwnTest(t *testing.T) {
	require.NotEmpty(t, Drops())
	require.NotEmpty(t, Rewrites())
	require.NotEmpty(t, Patterns())
	require.NotEmpty(t, Flags())
	for _, d := range Drops() {
		assert.NotEmpty(t, d.Test, "<drop word=%q> declares no test", d.Word)
	}
	for _, r := range Rewrites() {
		assert.NotEmpty(t, r.Test, "<rewrite from=%q> declares no test", r.From)
	}
	for _, p := range Patterns() {
		assert.NotEmpty(t, p.Test, "<pattern match=%q> declares no test", p.Match)
		assert.NotEmpty(t, p.Expect, "<pattern match=%q> declares no expect", p.Match)
	}
	for _, f := range Flags() {
		assert.NotEmpty(t, f.Test, "<flag phrase=%q> declares no test", f.Phrase)
	}
}

// A pattern carrying an id is a rule a caller selects by name, so the names
// have to be there for --only to reach them.
func TestThePatternsCarryTheRuleNames(t *testing.T) {
	ids := PatternIDs()

	require.NotEmpty(t, ids)
	assert.Contains(t, ids, "tombstones/former-state")
	assert.Contains(t, ids, "tombstones/date")
}

// Apply is the whole of what a pattern does, so it is asserted directly rather
// than only through a caller.
func TestAPatternRewritesWhatItMatches(t *testing.T) {
	for _, p := range Patterns() {
		if p.ID == "tombstones/date" {
			assert.NotContains(t, p.Apply("Verified on 2026-01-02 by hand"), "2026")
			return
		}
	}
	t.Fatal("the table carries no date pattern")
}

func TestAppliesTo(t *testing.T) {
	assert.True(t, AppliesTo("", "message"))
	assert.True(t, AppliesTo("", "comment"))
	assert.True(t, AppliesTo("both", "message"))
	assert.True(t, AppliesTo("message", "message"))
	assert.False(t, AppliesTo("message", "comment"))
	assert.False(t, AppliesTo("comment", "message"))
}
