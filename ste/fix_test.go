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
