package askproperly

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kinds(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Kind)
	}
	return out
}

func TestAProseQuestionIsFound(t *testing.T) {
	for _, msg := range []string{
		"Which one do you want?",
		"Should I land these now?",
		"What do you want to do with these two repos?",
		"The layout is unpinned. Is that a contract, or an illustration?",
		"Do you want me to pick the strict rule here?",
	} {
		hits := FindQuestions(msg)
		require.NotEmpty(t, hits, "expected a finding in %q", msg)
	}
}

func TestADeferralWithNoQuestionMarkIsFound(t *testing.T) {
	for _, msg := range []string{
		"Four questions remain. Your call.",
		"I won't touch them until you say.",
		"Let me know and I'll land the rest.",
		"Tell me which way you want it and I'll write it up.",
		"Answer them here, or say \"your call\" and I'll pick.",
	} {
		hits := FindQuestions(msg)
		require.NotEmpty(t, hits, "expected a finding in %q", msg)
	}
}

// The incident this plugin exists for: a closing message that lists open
// decisions and invites the user to answer in prose.
func TestTheIncidentMessageIsRefused(t *testing.T) {
	msg := "Eight decisions landed, PR green, and four questions still open.\n\n" +
		"1. Enum inside a collection - one is wrong.\n" +
		"2. Unknown CLI option - exit 64, or into raw_args?\n\n" +
		"Answer them here, or say \"your call\" and I'll pick, land them, and empty the file."
	hits := FindQuestions(msg)
	require.NotEmpty(t, hits)
	assert.Contains(t, kinds(hits), "question")
	assert.Contains(t, kinds(hits), "deferral")
}

// A "?" is not enough on its own. This org's specs are full of nullable types
// and every compare URL carries a query string; matching a bare "?" reports a
// question in a message that asked nothing.
func TestProseThatMustNotTrip(t *testing.T) {
	for _, msg := range []string{
		"find and index_of now return UInt? rather than Int?.",
		"A List<Int?> sorts with null compared as the type's default value.",
		"Pushed. The compare page is https://github.com/o/r/compare/master...claude/x?expand=1 and CI is green.",
		"See [the compare page](https://github.com/o/r/compare/a...b?expand=1) for the diff.",
		"Landed the rule in docs/json.md and pinned it in tests/json.dats. CI is green.",
		"I reverted the edit. Nothing was pushed.",
		"The declaration is `String? find(String s)` and the field is `Int? n`.",
	} {
		assert.Empty(t, FindQuestions(msg), "expected no finding in %q", msg)
	}
}

func TestFencedAndQuotedTextIsExempt(t *testing.T) {
	fenced := "Here is the rule I wrote:\n\n```\nShould I land this?\n```\n\nIt is committed."
	assert.Empty(t, FindQuestions(fenced))

	quoted := "The snippet says:\n\n> Do you want me to fix it?\n\nSo I fixed it."
	assert.Empty(t, FindQuestions(quoted))

	indented := "The doc reads:\n\n    Which one should win?\n\nI picked the first."
	assert.Empty(t, FindQuestions(indented))
}

// Matching the siblings: a question in inline backticks is still a question.
func TestInlineBackticksAreNotExempt(t *testing.T) {
	assert.NotEmpty(t, FindQuestions("So: `which one do you want?`"))
}

func TestEmptyMessageIsAllowed(t *testing.T) {
	assert.Empty(t, FindQuestions(""))
	assert.Empty(t, FindQuestions("   \n\t "))
}

func TestAHitCarriesItsLine(t *testing.T) {
	hits := FindQuestions("Landed the fix.\n\nWhich rule should win?\n\nCI is green.")
	require.Len(t, hits, 1)
	assert.Equal(t, "question", hits[0].Kind)
	assert.Equal(t, "Which rule should win?", hits[0].Line)
}

// A question mark that closes a sentence with no interrogative cue is not a
// question this plugin reports.
func TestAQuestionMarkWithNoCueIsIgnored(t *testing.T) {
	assert.Empty(t, FindQuestions("The field is Int? and the token is a Float."))
}

func TestFindingsAreDedupedPerLine(t *testing.T) {
	hits := FindQuestions("Which one? Which one? Which one?")
	assert.Len(t, hits, 1)
}

// repairMessages is every message the package's tests flag. Repair must leave
// each with nothing left to report.
func repairMessages() []string {
	incident := "Eight decisions landed, PR green, and four questions still open.\n\n" +
		"1. Enum inside a collection - one is wrong.\n" +
		"2. Unknown CLI option - exit 64, or into raw_args?\n\n" +
		"Answer them here, or say \"your call\" and I'll pick, land them, and empty the file."
	return []string{
		"Which one do you want?",
		"Should I land these now?",
		"What do you want to do with these two repos?",
		"The layout is unpinned. Is that a contract, or an illustration?",
		"Do you want me to pick the strict rule here?",
		"Four questions remain. Your call.",
		"I won't touch them until you say.",
		"Let me know and I'll land the rest.",
		"Tell me which way you want it and I'll write it up.",
		"Answer them here, or say \"your call\" and I'll pick.",
		incident,
		"So: `which one do you want?`",
		"Which one? Which one? Which one?",
		"Landed the fix.\n\nWhich rule should win?\n\nCI is green.",
		"Four are still open. Which one should win?",
		"Putting the four decisions to you. Which do you prefer?",
		"Done. Which rule should win?",
		"Your call. Let me know. Up to you. Shall I? Want me to?",
		"Want me to fix it?",
		"Should I " + strings.Repeat("really ", 40) + "land it?",
		"Landed the fix. Which rule\nshould win?",
		"Your call.",
		"Your call.\n",
	}
}

func TestRepairClearsEveryFlaggedMessage(t *testing.T) {
	for _, msg := range repairMessages() {
		require.NotEmpty(t, FindQuestions(msg), "the sample must start flagged: %q", msg)
		assert.Empty(t, FindQuestions(Repair(msg)), "still flagged after repair: %q", msg)
	}
}

func TestRepairTurnsAQuestionIntoAStatement(t *testing.T) {
	assert.Equal(t, "I will use the cache or the store.", Repair("Should I use the cache or the store?"))
	assert.Equal(t, "The store holds the rows.", Repair("Which store holds the rows?"))
	assert.Equal(t, "I will pick the strict rule here.", Repair("Do you want me to pick the strict rule here?"))
	assert.Equal(t, "I will fix it.", Repair("Want me to fix it?"))
	assert.Equal(t, "I will land these now.", Repair("Should I land these now?"))
	assert.Equal(t, "The one. The one. The one.", Repair("Which one? Which one? Which one?"))
	assert.NotContains(t, Repair("Which one do you want?"), "?")
}

// Every deferral phrase in the table gets a repair, and the repair carries no
// phrase of its own back into the message.
func TestEveryDeferralPhraseHasARepair(t *testing.T) {
	for _, phrase := range deferralPhrases {
		own := owningPhrase[phrase]
		require.NotEmpty(t, own, "no owning phrase for %q", phrase)
		for _, other := range deferralPhrases {
			assert.NotContains(t, strings.ToLower(own), other, "the repair for %q carries %q", phrase, other)
		}
	}
}

// The table drives the repair, so a phrase added without one fails here.
func TestRepairRewritesEveryDeferralPhrase(t *testing.T) {
	for _, phrase := range deferralPhrases {
		msg := "Done. " + phrase + "."
		require.NotEmpty(t, FindQuestions(msg), "the sample must start flagged: %q", msg)
		repaired := Repair(msg)
		assert.Empty(t, FindQuestions(repaired), "still flagged after repairing %q: %q", phrase, repaired)
		assert.NotContains(t, strings.ToLower(repaired), phrase, "the phrase %q survived: %q", phrase, repaired)
	}
}

// Nothing outside the replaced span moves: the whitespace and the blank lines
// around it are byte-identical.
func TestRepairKeepsTheBytesAroundAReplacedSpan(t *testing.T) {
	assert.Equal(t, "Before. mine to decide. After.", Repair("Before. Your call. After."))
	assert.Equal(t, "Before. The one. After.", Repair("Before. Which one? After."))
	assert.Equal(t, "Line one.\n\nmine to decide.\n\nLine three.", Repair("Line one.\n\nYour call.\n\nLine three."))
}

// A quoted or fenced question asked nothing, so the repair does not reach it.
func TestRepairLeavesExemptTextAlone(t *testing.T) {
	for _, msg := range []string{
		"Here is the rule I wrote:\n\n```\nShould I land this?\n```\n\nIt is committed.",
		"The snippet says:\n\n> Do you want me to fix it?\n\nSo I fixed it.",
		"The doc reads:\n\n    Which one should win?\n\nI picked the first.",
		"The snippet says \"your call\" and stops.",
		"find and index_of now return UInt? rather than Int?.",
	} {
		assert.Equal(t, msg, Repair(msg), "exempt text is not repaired: %q", msg)
	}
}

func TestRepairLeavesAnEmptyMessage(t *testing.T) {
	assert.Equal(t, "", Repair(""))
	assert.Equal(t, "   \n\t ", Repair("   \n\t "))
}
