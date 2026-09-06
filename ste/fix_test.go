package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfmt/ste"
)

func TestFixExpandsAContraction(t *testing.T) {
	assert.Equal(t, "It does not run.", ste.Fix("It doesn't run."))
}

func TestFixKeepsTheCapitalTheSourceUsed(t *testing.T) {
	assert.Equal(t, "Do not run it. It is late.", ste.Fix("Don't run it. It's late."))
}

func TestFixWritesTheApprovedModal(t *testing.T) {
	assert.Equal(t, "A caller must wait, and it can fail.", ste.Fix("A caller should wait, and it might fail."))
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

// A semicolon inside an inline code span is what the sentence documents.
func TestFixLeavesAnInlineCodeSpanAlone(t *testing.T) {
	assert.Equal(t, "Write `a; b` for it. It does not run.", ste.Fix("Write `a; b` for it. It doesn't run."))
}

// A link's target is a URL. A word inside it is not prose.
func TestFixLeavesALinkTargetAlone(t *testing.T) {
	assert.Equal(t, "The [it is](https://x/it's) page is not prose.", ste.Fix("The [it's](https://x/it's) page is not prose."))
}

// Splitting a long sentence needs a writer who knows which half is the point.
func TestFixLeavesALongSentenceAlone(t *testing.T) {
	long := "The gate reads every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own."
	assert.Equal(t, long, ste.Fix(long))
}

func TestFixLeavesCleanProseAlone(t *testing.T) {
	clean := "A short sentence that breaks no rule."
	assert.Equal(t, clean, ste.Fix(clean))
}

// A repair that leaves its own finding standing loops the caller forever.
func TestFixClearsTheMechanicalFindings(t *testing.T) {
	text := "It doesn't matter; a caller should wait, so the gate clears."
	assert.NotEmpty(t, ste.Check(text, 1))
	assert.Empty(t, ste.Check(ste.Fix(text), 1))
}
