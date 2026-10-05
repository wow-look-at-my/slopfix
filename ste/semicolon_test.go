package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Semicolons from a real repository. A period replaces one only where each
// side is a sentence. A semicolon between items or phrases stays, and its
// finding asks for a rewrite by hand.
var realSemicolons = []struct {
	name, in, want string
}{
	{
		"items of an e.g. list stay",
		"Defining mechanics are the PRIMARY behaviors without which the deliverable is NOT recognizably that thing — e.g. for a key-value store, get-after-set; for a parser, round-trip of valid input; for a platformer, enemies that defeat / are defeated by the player plus a win state and a lose state.",
		"",
	},
	{
		"phrases in parentheses stay",
		"Usually: a tangled unit that can't be tested in isolation (every fix breaks something else); test theater (tests that don't drive the real shipped path); or a subsystem whose design fights the objective and needs a clean rewrite.",
		"",
	},
	{
		"a noun phrase after the semicolon stays",
		"`{CHAT_HISTORY}.jsonl` — the implementer's transcript and the verifier's inlined gap feedback; richest signal for the whack-a-mole pattern.",
		"",
	},
	{
		"a bare noun is no subject",
		"- `analysis` — understand existing code; deliverable is prose, diff may be empty.",
		"",
	},
	{
		"a label with no verb before the semicolon stays",
		"- `{SKEPTIC_SCRATCH}` — yours, for cheap spot-checks only; when one re-runs the `## Verification plan`, the literal `{SCRATCH}` placeholder resolves here.",
		"",
	},
	{
		"a clause on each side divides",
		"Do NOT modify the workspace; your only write is `{PLAN_FILE}`.",
		"Do NOT modify the workspace. Your only write is `{PLAN_FILE}`.",
	},
	{
		"an imperative after a clause divides",
		"The implementer sees only a short pointer to your note; write for it.",
		"The implementer sees only a short pointer to your note. Write for it.",
	},
	{
		"a subject and its verb after an instruction divide",
		"Call `{GOAL_TOOL}(completed: true, message: \"summary\")` when done; the harness verifies what is complete and tells you what is missing on the next nudge.",
		"Call `{GOAL_TOOL}(completed: true, message: \"summary\")` when done. The harness verifies what is complete and tells you what is missing on the next nudge.",
	},
	{
		"a gerund subject the parser cannot read stays",
		"Raising a fresh nitpick each round while the criteria hold is the failure mode that makes goals unfinishable; when every prior gap is fixed and every gating criterion holds, return `Not Refuted`.",
		"",
	},
	{
		"a subordinate clause and its main clause divide",
		"The verifier reads the run log; when a gap is fixed, the verifier returns `Not Refuted`.",
		"The verifier reads the run log. When a gap is fixed, the verifier returns `Not Refuted`.",
	},
}

func TestARealSemicolonComesOutGrammaticalOrAsWritten(t *testing.T) {
	only := func(id string) bool { return id == ste.IDSemicolon }
	for _, c := range realSemicolons {
		t.Run(c.name, func(t *testing.T) {
			want := c.want
			if want == "" {
				want = c.in
			}
			got := ste.FixSelected(c.in, only)
			assert.Equal(t, want, got)
			for _, f := range ste.Check(got, 1) {
				if f.ID == ste.IDSemicolon {
					assert.Equal(t, ste.FixSemicolonByHand, f.Fix, "a semicolon the repair leaves asks for a rewrite by hand")
				}
			}
		})
	}
}

// A repair that leaves no semicolon reports none, and one that keeps a
// semicolon marks it as no repair's to make.
func TestAKeptSemicolonIsNotRepairable(t *testing.T) {
	got := findings("Each rule reads a source: for the build, the cache; for the test, the tree.", ste.IDSemicolon)
	if assert.Len(t, got, 1) {
		assert.True(t, ste.ByHand(got[0].Fix))
	}
	assert.False(t, ste.ByHand("Write a period and start a new sentence."))
}
