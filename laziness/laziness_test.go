package laziness_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/laziness"
)

// The incident this rule exists for. A session diagnosed a real defect, wrote it
// up, and closed the turn without repairing it.
func TestAReportedDefectLeftUnfixedIsAPunt(t *testing.T) {
	hits := laziness.Check("I found a broken build phase. Neither mine to fix unasked.")
	require.NotEmpty(t, hits)
	assert.Equal(t, laziness.ID, hits[0].ID)
	assert.Contains(t, hits[0].Sentence, "mine to fix")
}

// The other half of the fold. A turn that ends on a question whose answer was
// already yes.
func TestAskingPermissionInPlaceOfActingIsAPunt(t *testing.T) {
	for _, message := range []string{
		"The analyzer has a bug. Want me to fix it?",
		"Should I go ahead and push?",
		"I can port the fix if you'd like.",
		"Say the word.",
	} {
		assert.NotEmpty(t, laziness.Check(message), "no finding on %q", message)
	}
}

// Every tell must be reachable. A row nothing can fire is a row that reads as
// coverage.
func TestEveryShapeInTheTableIsReported(t *testing.T) {
	for _, message := range []string{
		"That is not my problem.",
		"Worth your attention.",
		"I left it as-is.",
		"That is out of scope.",
		"It is pre-existing, so I did not touch it.",
		"I did not break it.",
		"It fails on master too, so nothing here caused it.",
		"Whether master is red too is the question.",
	} {
		assert.NotEmpty(t, laziness.Check(message), "no finding on %q", message)
	}
}

// The negative controls. A message that owns the work carries no finding.
func TestOwnedWorkIsNotAPunt(t *testing.T) {
	for _, message := range []string{
		"The build phase was broken. I fixed it and pushed the fix.",
		"The failure was pre-existing, and I already fixed it.",
		"This needs your call on A against B, so I pushed the branch with A and left the test red.",
		"The rule reads a pre-existing file.",
		"I rewrote the loader and the tests pass.",
	} {
		assert.Empty(t, laziness.Check(message), "false finding on %q", message)
	}
}

// A pardon is per sentence. The repair has to be claimed where the tell sits.
func TestAPardonInAnotherSentenceDoesNotExcuseThePunt(t *testing.T) {
	assert.NotEmpty(t, laziness.Check("I pushed the fix for the loader. The other defect is not my problem."))
}

// Quoting the policy must not trip it. Fenced code, indented code and a
// blockquote are what a message quotes rather than asserts.
func TestQuotedTextIsExempt(t *testing.T) {
	assert.Empty(t, laziness.Check("The rule catches this:\n\n```\nWant me to fix it?\n```\n"))
	assert.Empty(t, laziness.Check("> Want me to fix it?"))
	assert.Empty(t, laziness.Check("The shape is:\n\n    Want me to fix it?\n"))
}

// An inline span is NOT exempt, matching the sibling guards.
func TestAnInlineSpanIsNotExempt(t *testing.T) {
	assert.NotEmpty(t, laziness.Check("The defect is real. `Want me to fix it?`"))
}

// A period inside a token never ends a sentence, so the quoted sentence reads
// whole rather than torn at a file name.
func TestASentenceIsNotCutAtAFileName(t *testing.T) {
	hits := laziness.Check("The budget in CLAUDE.md is broken and that is not my problem.")
	require.NotEmpty(t, hits)
	assert.Contains(t, hits[0].Sentence, "CLAUDE.md")
}

// The reported line is the line the sentence really sits on.
func TestTheFindingNamesItsLine(t *testing.T) {
	hits := laziness.Check("All done.\nThe tests pass.\nThat is out of scope.")
	require.NotEmpty(t, hits)
	assert.Equal(t, 3, hits[0].Line)
}

// A sentence carrying several shapes still yields a single finding.
func TestASentenceIsReportedForASingleTell(t *testing.T) {
	hits := laziness.Check("That is out of scope and not my problem.")
	assert.Len(t, hits, 1)
}

// An empty message answers with nothing rather than with a panic.
func TestAnEmptyMessageIsClean(t *testing.T) {
	assert.Empty(t, laziness.Check(""))
}
