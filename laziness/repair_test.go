package laziness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tellSamples is one worked sentence per row of tells, in the same order. A row
// added to the table without a sample and a repair fails the length checks, so
// the table itself drives the proof.
var tellSamples = []string{
	"Neither mine to fix.",
	"That is not my problem.",

	"That is worth your attention.",
	"I am flagging it for you.",
	"Someone should fix this.",
	"You may want to fix this.",

	"I left it as-is.",
	"I am leaving it alone.",
	"I did not fix it.",

	"It happened unasked.",
	"That is out of scope.",
	"That is outside my scope.",
	"It is not caused by my change.",
	"It is pre-existing, so I did not touch it.",

	"Three things to establish.",
	"Whether master is red too is the question.",
	"Figure out whether I introduced it.",
	"This is not this PR's fault.",
	"It fails on master too, so nothing here caused it.",
	"I did not break it.",

	"Want me to fix it?",
	"Would you like me to fix it?",
	"Shall I port it?",
	"Should I go ahead and push?",
	"Do you want me to fix it?",
	"Let me know if you'd like.",
	"I can port the fix if you'd like.",
	"Say the word.",
}

// Every tell row has a repair, its sample is rewritten, and the result is
// clean. This is the guard that a new row cannot arrive without one.
func TestEveryTellRowIsRepaired(t *testing.T) {
	require.Len(t, repairs, len(tells), "a row of tells has no repair")
	require.Len(t, tellSamples, len(tells), "a row of tells has no sample")
	for n, tr := range tells {
		message := tellSamples[n]
		require.NotEmpty(t, Check(message), "row %d (%s) does not fire on its sample", n, tr.name)
		repaired := Repair(message)
		assert.NotEqual(t, message, repaired, "row %d (%s) left %q as written", n, tr.name, message)
		assert.Empty(t, Check(repaired), "row %d (%s) still fires on %q", n, tr.name, repaired)
	}
}

// A repaired message is clean for every message the package's tests carry.
func TestEveryFindingIsRepaired(t *testing.T) {
	for _, message := range []string{
		"I found a broken build phase. Neither mine to fix unasked.",
		"The analyzer has a bug. Want me to fix it?",
		"Should I go ahead and push?",
		"I can port the fix if you'd like.",
		"Say the word.",
		"That is not my problem.",
		"Worth your attention.",
		"I left it as-is.",
		"That is out of scope.",
		"It is pre-existing, so I did not touch it.",
		"I did not break it.",
		"It fails on master too, so nothing here caused it.",
		"Whether master is red too is the question.",
		"I pushed the fix for the loader. The other defect is not my problem.",
		"The defect is real. `Want me to fix it?`",
		"The budget in CLAUDE.md is broken and that is not my problem.",
		"All done.\nThe tests pass.\nThat is out of scope.",
	} {
		assert.Empty(t, Check(Repair(message)), "repair of %q still fires", message)
	}
}
