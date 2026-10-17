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

// An offer held back for a reply is permission seeking. The Stop hook answers
// it with the approval the reader would type.
func TestAnOfferHeldForAReplyIsAPunt(t *testing.T) {
	hits := laziness.Check("The loader fix is ready. Tell me if you want it.")
	require.Len(t, hits, 1)
	assert.Equal(t, "asking permission in place of acting", hits[0].Tell)
	assert.Equal(t, "Tell me if you want it.", hits[0].Sentence)
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
