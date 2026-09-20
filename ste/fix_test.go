package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

func TestFixExpandsAContraction(t *testing.T) {
	assert.Equal(t, "It does not run.", ste.Fix("It doesn't run."))
}

func TestFixKeepsTheCapitalTheSourceUsed(t *testing.T) {
	assert.Equal(t, "Do not run it. It is late.", ste.Fix("Don't run it. It's late."))
}

func TestFixWritesTheApprovedModal(t *testing.T) {
	assert.Equal(t, "A caller must wait. It can fail.", ste.Fix("A caller should wait. It might fail."))
}

func TestFixWritesAPeriodForASemicolon(t *testing.T) {
	assert.Equal(t, "The gate is shut. The write fails.", ste.Fix("The gate is shut; the write fails."))
}

func TestFixWritesAPeriodForATrailingSemicolon(t *testing.T) {
	assert.Equal(t, "The gate is shut.", ste.Fix("The gate is shut;"))
}

func TestFixWritesAPeriodForACommaSplice(t *testing.T) {
	assert.Equal(t, "The gate is shut. So the write fails.", ste.Fix("The gate is shut, so the write fails."))
}

// A conjunction after the comma joins equals, so that comma splices whatever
// follows it. checkSplices reports it, and the repair covers what it reports.
func TestFixBreaksASpliceThatCarriesAConjunction(t *testing.T) {
	assert.Equal(t, "A caller waits. And it can fail.", ste.Fix("A caller waits, and it might fail."))
}

// A bare comma splices only when what precedes it already stands alone.
// checkSplices makes that test, and the repair makes the same test.
func TestFixLeavesAnIntroductoryCommaAlone(t *testing.T) {
	phrase := "Under the gate, the write fails."
	assert.Equal(t, phrase, ste.Fix(phrase))
}

// A list carries no verb after a comma, so a comma in it is not a splice.
func TestFixLeavesAListAlone(t *testing.T) {
	list := "The rules cover a semicolon, a modal, and a splice."
	assert.Equal(t, list, ste.Fix(list))
}

// An HTML entity ends in a semicolon that belongs to the entity.
func TestFixLeavesAnEntityAlone(t *testing.T) {
	assert.Equal(t, "Write &amp; for it. It is data.", ste.Fix("Write &amp; for it. It's data."))
}

// A semicolon inside an inline code span is what the sentence documents.
func TestFixLeavesAnInlineCodeSpanAlone(t *testing.T) {
	assert.Equal(t, "Write `a; b` for it. It does not run.", ste.Fix("Write `a; b` for it. It doesn't run."))
}

// A link's target is a URL. A word inside it is not prose.
func TestFixLeavesALinkTargetAlone(t *testing.T) {
	assert.Equal(t, "The [it is](https://x/it's) page is not prose.", ste.Fix("The [it's](https://x/it's) page is not prose."))
}

// A coordinator joining verbs that share a subject is the weaker seam, and the
// division is taken anyway: the cap is a limit, and a finding no repair answers
// costs a reader the whole file. The conjunction stays and opens the new
// sentence, so the division loses no word.
func TestFixDividesAtASharedSubjectWhenNothingBetterIsThere(t *testing.T) {
	long := "The gate reads every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own."
	assert.Equal(t,
		"The gate reads every file in the session. And refuses the write when any one of them carries a finding that a rewrite cannot repair on its own.",
		ste.Fix(long))
}

// The seam a writer would have used wins over the bare a single beside it.
func TestFixPrefersAClauseSeamToAWordBoundary(t *testing.T) {
	long := "The loader opens the file and reads every row it holds into memory, which is the whole reason a caller waits on it before the header check runs."
	assert.Equal(t,
		"The loader opens the file and reads every row it holds into memory. Which is the whole reason a caller waits on it before the header check runs.",
		ste.Fix(long))
}

// With no conjunction, no comma and no clause boundary anywhere, the division
// falls to a bare gap between words. Awkward, and under the cap.
func TestFixDividesASentenceCarryingNoSeamAtAll(t *testing.T) {
	long := "A reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a repair from the tool rather than a deletion."
	fixed := ste.Fix(long)
	assert.NotEqual(t, long, fixed)
	assert.Empty(t, ste.Check(fixed, 1))
}

// A division never lands inside an inline code span, so the span survives the
// repair exactly as the source wrote it.
func TestFixDividesALongSentenceAroundACodeSpan(t *testing.T) {
	long := "The gate reads `a; b` out of every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "`a; b`")
	assert.Empty(t, ste.Check(fixed, 1))
}

// A parenthetical counts as a single word, so a division inside it would halve
// something STE says is indivisible.
func TestFixDividesALongSentenceAroundAParenthetical(t *testing.T) {
	long := "The gate reads every file in the session (the header, the body and the trailer alike) and refuses the write when any one of them carries a finding nothing repairs."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "(the header, the body and the trailer alike)")
	assert.Empty(t, ste.Check(fixed, 1))
}

// A coordinator joining clauses that each name who acts IS a seam.
func TestFixDividesAtAClauseThatNamesWhoActs(t *testing.T) {
	long := "The gate reads every file in the session and the write fails when any one of them carries a finding that a rewrite cannot repair on its own."
	assert.Equal(t,
		"The gate reads every file in the session. The write fails when any one of them carries a finding that a rewrite cannot repair on its own.",
		ste.Fix(long))
}

func TestFixLeavesCleanProseAlone(t *testing.T) {
	clean := "A short sentence that breaks no rule."
	assert.Equal(t, clean, ste.Fix(clean))
}

// A repair that leaves its own finding standing loops the caller forever.
func TestFixClearsTheMechanicalFindings(t *testing.T) {
	text := "It doesn't matter; a caller should wait, so the write fails."
	assert.NotEmpty(t, ste.Check(text, 1))
	assert.Empty(t, ste.Check(ste.Fix(text), 1))
}
