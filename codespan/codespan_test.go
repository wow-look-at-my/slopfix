package codespan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// spans answers the text of each span Find reports in s.
func spans(s string) []string {
	var out []string
	for _, sp := range Find(s) {
		out = append(out, s[sp[0]:sp[1]])
	}
	return out
}

// A backtick that closes a word with no opener before it is a stray. It does
// not pair with the opener of the span after it, so each later span stays whole.
func TestAStrayCloserPairsWithNothing(t *testing.T) {
	s := "Their jobs run .github/dats-fixtures/cache-profile.dats` asserts the tripwires, run by the dats action in `host-build` the same way `same.dats` and `smoke.dats`."
	assert.Equal(t, []string{"`host-build`", "`same.dats`", "`smoke.dats`"}, spans(s))
	assert.Len(t, Stray(s), 1)
}

// A run of backticks closes only on a run of the same length.
func TestARunClosesOnARunOfItsLength(t *testing.T) {
	assert.Equal(t, []string{"``a ` b``", "`c`"}, spans("``a ` b`` and `c`"))
}

// An opener followed by a blank opens nothing.
func TestAnOpenerBeforeABlankIsAStray(t *testing.T) {
	s := "a ` b `c`"
	assert.Equal(t, []string{"`c`"}, spans(s))
	assert.Len(t, Stray(s), 1)
}
