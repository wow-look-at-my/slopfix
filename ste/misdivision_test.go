package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Comments from a real tree that a carrier once cut into broken English. Each
// bad text is what the carrier wrote. A cut between words, the last resort, is
// not what these cases guard.
var misdivisions = []struct {
	name, in string
	bad      []string
}{
	{
		"a phrase after be and its adverb stays with be",
		"The turn loop calls this immediately before `drain_pending_interjections`, which is immediately before each model request — the earliest point the model can see the text without cancelling anything.",
		[]string{"is immediately.", "this immediately.", "This happens before"},
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
		"a dash before and opens a sentence of its own",
		"Anything that lands while the session is busy is invisible to the entry gate -- and a session that is doing work is busy nearly all the time.",
		[]string{"a session. That is"},
	},
	{
		"a clause after a fronted phrase is the main clause",
		"On a bridge/web surface every inbound user message goes through the queue, so anything that lands while the session is busy is invisible to the entry gate -- and a session that is doing work is busy nearly all the time.",
		[]string{"queue, so anything"},
	},
	{
		"a lone pronoun object keeps its noun",
		"This covers a subordinator with no finite verb after it, as in \"reports whether the words\", or a noun followed by a new subject with no verb after it, as in \"answers the row the earliest node\".",
		[]string{"This happens after it", "covers as in"},
	},
}

// Sentences from a real tree that no grammatical division reads. The cut
// between words once parted a subject from its verb or a verb from its
// particle in each. The bad text is that seam.
var hardCuts = []struct {
	in  string
	bad string
}{
	{"A crate that still carries hits names the lint in an inner attribute at the top of its crate root (`src/lib.rs`, `src/main.rs`, each `src/bin/*.rs`, `build.rs`, which are separate roots and inherit nothing), with the count it is carrying: `#![allow(clippy::string_slice)] // 4 hits predate the gate`.", "it. Is"},
	{"Immediate-sending the new prompt onto the server queue while the one stayed local ran them AHEAD of it (the merge is server-rows-first), so `[2, 3]` showed up as `[3, 2]`.", "showed. Up"},
	{"That snapshot must record every discovered skill name — including `paths:`-gated and preloaded skills that the listing baseline (`slash_skills`) holds back — so session-start telemetry can reuse it instead of re-walking the disk.", "telemetry. Can"},
	{"The item list a `/todo` capture sends on its SECOND model call: the same prepared snapshot `/btw` sends, its instruction, then the first response echoed back verbatim (reasoning included) with the tool result that answered it.", "echoed. Back"},
	{"The span the current window covers at `now`: the whole window once the response has run that long, and the time since its first chunk before then.", "first. Chunk"},
}

// A last-resort cut may leave a sentence whole when no cut reads. These cases
// assert only that the cut never lands on the seam it once took.
func TestALastResortCutKeepsAVerbWithItsNeighbours(t *testing.T) {
	for _, c := range hardCuts {
		t.Run(c.bad, func(t *testing.T) {
			assert.NotContains(t, ste.Fix(c.in), c.bad)
		})
	}
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
