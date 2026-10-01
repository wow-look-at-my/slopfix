package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/ste"
)

// The sentences the repair rewrites, and the sentences it declines, are worked
// examples in the rules folder. What is left here is what such an example
// cannot say: an invariant the repair holds whatever it writes.

// Leading closes a sentence at a clause boundary the parser finds, so the head it
// keeps is a sentence under the cap and never a cut at a word.
func TestLeadingClosesAtAClauseBoundary(t *testing.T) {
	long := "The comment scan reads each file that the branch changed since its merge base, and it rewrites every number it finds in a comment into words that stay true."
	head, ok := ste.Leading(long)
	require.True(t, ok)
	assert.Equal(t, "The comment scan reads each file that the branch changed since its merge base.", head)
	assert.LessOrEqual(t, ste.WordCount(head), ste.SentenceWordCap)
	assert.Empty(t, ste.Check(head, 1))

	_, ok = ste.Leading("a very long run of words with no verb and no boundary of any kind at all anywhere in it whatsoever here")
	assert.False(t, ok, "no clause boundary, so no head")
}

// With no clause boundary, a division writes a fragment. The repair leaves the
// sentence whole, and Check still reports it for a person to rewrite.
func TestFixLeavesASentenceWithNoClauseBoundaryForAPerson(t *testing.T) {
	long := "A reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a repair from the tool rather than a deletion."
	fixed := ste.Fix(long)
	assert.Equal(t, long, fixed)
	findings := ste.Check(fixed, 1)
	require.Len(t, findings, 1)
	assert.Equal(t, ste.IDSentenceCap, findings[0].ID)
}

// A division never lands inside an inline code span, so the span survives the
// repair exactly as the source wrote it.
func TestFixDividesALongSentenceAroundACodeSpan(t *testing.T) {
	long := "The gate reads `a; b` out of every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "`a; b`")
	assert.Empty(t, ste.Check(fixed, 1))
}

// The parser reads a masked code span, whose filler word ends before the closing
// backtick. A division after the span must still keep the whole span.
func TestFixKeepsTheCodeSpanThatEndsTheLeftSentence(t *testing.T) {
	long := "The tarball is rooted at `./` and unpacks *as* the build directory — extracting it without `-C dir` sprays `src/`, `include/` and a foreign `.gitignore` over the repo root and chowns it."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "rooted at `./`")
	assert.Contains(t, fixed, "*as*")
	assert.NotContains(t, fixed, "Unpacks")
}

// ", and" before a subordinate clause and its main clause is a sentence boundary.
func TestFixDividesBeforeASubordinateClauseAfterAnd(t *testing.T) {
	long := "This closed a real hole: `a_test.go` is `//go:build x`, and for as long as the gate ran default tags only, its violations were invisible and its tests compiled nowhere."
	fixed := ste.Fix(long)
	assert.Equal(t, "This closed a real hole: `a_test.go` is `//go:build x`. For as long as the gate ran default tags only, its violations were invisible and its tests compiled nowhere.", fixed)
	assert.Empty(t, ste.Check(fixed, 1))

	long = "The gate reads every file that the session wrote, but if the cache is cold at the start of the run, the build waits for the whole tree."
	assert.Equal(t, "The gate reads every file that the session wrote. However, if the cache is cold at the start of the run, the build waits for the whole tree.", ste.Fix(long))
}

// A semicolon that ends the prose before a code span keeps its space.
func TestFixKeepsTheSpaceBeforeACodeSpan(t *testing.T) {
	assert.Equal(t, "Docker is unavailable. `MESA_DIR` still overrides it.", ste.Fix("Docker is unavailable; `MESA_DIR` still overrides it."))
}

// A parenthetical counts as a single word, so a division inside it would halve
// something STE says is indivisible.
func TestFixDividesALongSentenceAroundAParenthetical(t *testing.T) {
	long := "The gate reads every file in the session (the header, the body and the trailer alike) and refuses the write when any one of them carries a finding nothing repairs."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "(the header, the body and the trailer alike)")
	assert.Empty(t, ste.Check(fixed, 1))
}

// A repair that leaves its own finding standing loops the caller forever.
func TestFixClearsTheMechanicalFindings(t *testing.T) {
	text := "It doesn't matter; a caller should wait, so the write fails."
	assert.NotEmpty(t, ste.Check(text, 1))
	assert.Empty(t, ste.Check(ste.Fix(text), 1))
}
