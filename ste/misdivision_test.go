package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Comments from a real tree that the divider once cut into broken English. Each
// bad text is a fragment the division wrote, and no repair may write it again.
var misdivisions = []struct {
	name, in string
	bad      []string
}{
	{
		"a phrase after be and its adverb stays with be",
		"The turn loop calls this immediately before `drain_pending_interjections`, which is immediately before each model request — the earliest point the model can see the text without cancelling anything.",
		[]string{"is immediately.", "This happens before each"},
	},
	{
		"a rest that opens on a preposition has no subject",
		"Prints the fixed argv, and beside it the argv the pre-fix caller produced: the SAME builder for a plan with no fd, with the identical override appended to the finished command.",
		[]string{"Beside it the argv"},
	},
	{
		"a subject with a where clause is no subject to restate",
		"The dropdown sized its label column from the widest label under a 40-column cap while DISCARDING the ones above it, so a catalog where every id is long left nothing to take a max over: a zero-width column, and rows that draw, highlight and switch models with nothing written in them.",
		[]string{"where every id.", "That nothing"},
	},
	{
		"a phrase that a verb follows belongs to the subject",
		"carrier.go divides a long sentence where the words after the cut are no clause: a trailing adverbial, a phrase that describes a noun, or the rest of a list.",
		[]string{"where the words.", "This happens after the cut"},
	},
	{
		"a lone pronoun object keeps its noun",
		"This covers a subordinator with no finite verb after it, as in reports whether the words, or a noun followed by a new subject with no verb after it, as in answers the row the earliest node.",
		[]string{"no verb.", "This happens after it"},
	},
}

func TestADivisionNeverWritesABrokenFragment(t *testing.T) {
	for _, c := range misdivisions {
		t.Run(c.name, func(t *testing.T) {
			got := ste.Fix(c.in)
			for _, bad := range c.bad {
				assert.NotContains(t, got, bad)
			}
		})
	}
}
