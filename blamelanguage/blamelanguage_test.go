package blamelanguage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tells reports the phrase quoted by each hit, so a case asserts what fired.
func tells(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Sentence)
	}
	return out
}

// Every phrase in the table is reachable. A row nothing can reach is a row
// that reads as coverage and is not.
func TestEveryPhraseIsFound(t *testing.T) {
	for _, phrase := range phrases {
		hits := Check("The build is red. " + phrase + " so here we are.")
		require.NotEmpty(t, hits, "no hit for %q", phrase)
		assert.Equal(t, ID, hits[0].ID)
	}
}

func TestMatchingIgnoresCase(t *testing.T) {
	assert.NotEmpty(t, Check("This is PRE-EXISTING."))
	assert.NotEmpty(t, Check("Not My Problem."))
}

// A markdown line wrap splits a phrase. The collapsed text still matches, and
// the hit still names the line it started on.
func TestAPhraseSplitAcrossALineWrapIsFound(t *testing.T) {
	hits := Check("The suite is green.\nIt is worth your\nattention though.")
	require.NotEmpty(t, hits)
	assert.Equal(t, 2, hits[0].Line)
}

// The policy has to be writable. Fenced code, indented code and a blockquote
// are what a message quotes rather than asserts.
func TestQuotedTextIsExempt(t *testing.T) {
	for name, message := range map[string]string{
		"fenced":     "The rule bans this:\n```\npre-existing\n```\nand that is all.",
		"tilde":      "The rule bans this:\n~~~\nnot my problem\n~~~\nand that is all.",
		"indented":   "The rule bans this:\n\n    out of scope\n\nand that is all.",
		"blockquote": "The owner wrote:\n\n> someone should fix it\n\nand that is all.",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, Check(message))
		})
	}
}

// An inline backtick span is deliberately NOT exempt: a deflection in
// backticks is still the writer's own voice.
func TestInlineBackticksAreNotExempt(t *testing.T) {
	assert.NotEmpty(t, Check("The failure is `pre-existing` as far as I can tell."))
}

// Naming a real blocker plainly carries none of the phrases.
func TestAnHonestDeferralIsClean(t *testing.T) {
	assert.Empty(t, Check("This needs your call on A versus B, so I pushed the branch with A and left the test red."))
}

func TestAFixedAndOwnedReportIsClean(t *testing.T) {
	assert.Empty(t, Check("Found the null dereference in the parser, fixed it, and pushed. CI is green."))
}

// A phrase is reported at its earliest occurrence, however often the message
// repeats it.
func TestAPhraseIsReportedOnce(t *testing.T) {
	hits := Check("Not my problem. Really, not my problem.")
	assert.Len(t, hits, 1)
}

// The quoted sentence is bounded, so a report never pastes a paragraph back.
func TestTheQuotedLineIsBounded(t *testing.T) {
	hits := Check("Not my problem, " + strings.Repeat("and so on ", 40) + "the end.")
	require.NotEmpty(t, hits)
	assert.LessOrEqual(t, len(hits[0].Sentence), 160)
	assert.Contains(t, hits[0].Sentence, "...")
}

func TestAnEmptyMessageIsClean(t *testing.T) {
	assert.Empty(t, Check(""))
	assert.Empty(t, tells(Check("\n\n")))
}
