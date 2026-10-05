package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Semicolons from a real repository. A period replaces one where the words
// after it are a sentence. The items of a list after a colon each open a
// sentence behind "This also covers". Pairs of a case and its answer take a
// parenthesis, and an appositive takes a comma.
var realSemicolons = []struct {
	name, in, want string
}{
	{
		"the pairs of an e.g. list take parentheses",
		"Defining mechanics are the PRIMARY behaviors without which the deliverable is NOT recognizably that thing — e.g. for a key-value store, get-after-set; for a parser, round-trip of valid input; for a platformer, enemies that defeat / are defeated by the player plus a win state and a lose state.",
		"Defining mechanics are the PRIMARY behaviors without which the deliverable is NOT recognizably that thing — e.g. for a key-value store (get-after-set), for a parser (round-trip of valid input), and for a platformer (enemies that defeat / are defeated by the player plus a win state and a lose state).",
	},
	{
		"the items of a list after a colon each open a sentence",
		"Let it run if everything it does is ordinary development work on this machine: building, testing, searching, and editing project files; reading the user's own files, logs, configuration, and environment; scratch work in temp directories; read-only queries of the team's own services, dashboards, and internal APIs made from this machine (using stored credentials for read access is normal); git reads and commits (status, diff, log, show, add, commit, git rm of an already-committed file, switching branches).",
		"Let it run if everything it does is ordinary development work on this machine: building, testing, searching, and editing project files. This also covers reading the user's own files, logs, configuration, and environment. This also covers scratch work in temp directories. This also covers read-only queries of the team's own services, dashboards, and internal APIs made from this machine (using stored credentials for read access is normal). This also covers git reads and commits (status, diff, log, show, add, commit, git rm of an already-committed file, switching branches).",
	},
	{
		"the conjunction of the last item goes with its semicolon",
		"Usually: a tangled unit that can't be tested in isolation (every fix breaks something else); test theater (tests that don't drive the real shipped path); or a subsystem whose design fights the objective and needs a clean rewrite.",
		"Usually: a tangled unit that can't be tested in isolation (every fix breaks something else). This also covers test theater (tests that don't drive the real shipped path). This also covers a subsystem whose design fights the objective and needs a clean rewrite.",
	},
	{
		"a noun phrase after the semicolon is an appositive",
		"`{CHAT_HISTORY}.jsonl` — the implementer's transcript and the verifier's inlined gap feedback; richest signal for the whack-a-mole pattern.",
		"`{CHAT_HISTORY}.jsonl` — the implementer's transcript and the verifier's inlined gap feedback, richest signal for the whack-a-mole pattern.",
	},
	{
		"a clause with a bare noun for its subject divides",
		"- `analysis` — understand existing code; deliverable is prose, diff may be empty.",
		"- `analysis` — understand existing code. Deliverable is prose, diff may be empty.",
	},
	{
		"a subordinate clause after a label divides",
		"- `{SKEPTIC_SCRATCH}` — yours, for cheap spot-checks only; when one re-runs the `## Verification plan`, the literal `{SCRATCH}` placeholder resolves here.",
		"- `{SKEPTIC_SCRATCH}` — yours, for cheap spot-checks only. When one re-runs the `## Verification plan`, the literal `{SCRATCH}` placeholder resolves here.",
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
		"a subordinate clause and an instruction after a gerund subject divide",
		"Raising a fresh nitpick each round while the criteria hold is the failure mode that makes goals unfinishable; when every prior gap is fixed and every gating criterion holds, return `Not Refuted`.",
		"Raising a fresh nitpick each round while the criteria hold is the failure mode that makes goals unfinishable. When every prior gap is fixed and every gating criterion holds, return `Not Refuted`.",
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
				assert.NotEqual(t, ste.IDSemicolon, f.ID, "the repair leaves no semicolon: %s", f.Detail)
			}
		})
	}
}

// Every semicolon has a repair, so no finding asks for a rewrite by hand.
func TestEverySemicolonIsRepairable(t *testing.T) {
	in := "Each rule reads a source: for the build, the cache; for the test, the tree."
	got := findings(in, ste.IDSemicolon)
	if assert.Len(t, got, 1) {
		assert.False(t, ste.ByHand(got[0].Fix), got[0].Fix)
	}
	assert.Equal(t, "Each rule reads a source: for the build (the cache) and for the test (the tree).", ste.FixSelected(in, func(id string) bool { return id == ste.IDSemicolon }))
}
