package english

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The guard reads word classes, so a table that stopped declaring them would
// answer "noun" for every word and stand the rule down everywhere.
func TestTheTableDeclaresItsWordClasses(t *testing.T) {
	require.NotEmpty(t, loaded.Classes)
	assert.True(t, is("the", "article"))
	assert.True(t, is("when", "relative"))
	assert.True(t, is("because", "conjunction"))
	assert.True(t, is("nothing", "pronoun"))
	assert.True(t, is("element", "noun"))
	assert.False(t, is("derived", "noun"), "an -ed ending is a verb form")
	assert.False(t, is("was", "noun"), "an auxiliary is a closed class")
}

// A subject position that no pattern claims is a guard nothing runs.
func TestAPatternClaimsEachSubjectPosition(t *testing.T) {
	claimed := map[string]bool{}
	for _, p := range Patterns() {
		if p.Subject != "" {
			claimed[p.Subject] = true
		}
	}
	for _, position := range SubjectPositions() {
		assert.True(t, claimed[position], "no <pattern> declares subject=%q", position)
	}
}

// The sentence a clause sits in decides the reading, so the same words answer
// differently depending on what stands in front of them.
func TestTheSubjectDecidesWhetherAChangeVerbIsATombstone(t *testing.T) {
	for _, c := range []struct {
		name     string
		position string
		prose    string
		match    string
		want     bool
	}{
		{"opens the sentence", SubjectPreceding, "The retry loop was removed", " was", true},
		{"opens a later sentence", SubjectPreceding, "It holds. Documents were deleted", " were", true},
		{"a relative opens the clause", SubjectPreceding, "True when the element was added", " was", false},
		{"a conjunction opens the clause", SubjectPreceding, "It passes because nothing was added", " was", false},
		{"a noun carries the clause", SubjectPreceding, "The failure the whole rule was rewritten", " was", false},
		{"the match opens the sentence", SubjectMatched, "It replaced a line walk", "It replaced", true},
		{"a verb form stands before it", SubjectMatched, "The list is derived we removed", " we removed", true},
		{"a noun stands before it", SubjectMatched, "It names the ones it rewrote", " it rewrote", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			at := strings.Index(c.prose, c.match)
			require.GreaterOrEqual(t, at, 0, "the case names text the prose does not carry")
			assert.Equal(t, c.want, asserts(c.position, c.prose, at))
		})
	}
}
