package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An arity word states a design, not a tally. None of these goes stale when an
// item is added somewhere, which is the whole claim this rule reads a number as.
func TestAnArityWordCountsNothing(t *testing.T) {
	for _, line := range []string{
		"Number is a number standing on its own, as a comment writes one.",
		"The frames are read in turn, so the same phrase can match more than one of them.",
		"Command probe dumps the parse of one file, for diagnosis only.",
		"The rule names one command. Redirecting anything else is ordinary work.",
		"It is parsed once, at start, and a malformed table panics.",
		"quoted is what a report names: the phrase where the rule found one.",
	} {
		assert.Empty(t, Find(line, Comment), "flagged: %s", line)
	}
}

// The control. A possession verb makes it a claim about how many things are
// here, and that claim is wrong the moment another arrives.
func TestAPossessionVerbMakesArityACount(t *testing.T) {
	for _, line := range []string{
		"The map has one entry.",
		"There is one caller left.",
		"The payload carries one step.",
	} {
		assert.NotEmpty(t, Find(line, Comment), "not flagged: %s", line)
	}
}

// A real tally is untouched by any of this.
func TestARealCountIsStillReported(t *testing.T) {
	assert.NotEmpty(t, Find("The walk skips 13 retired plugins.", Comment))
	assert.NotEmpty(t, Find("This holds three frames.", Comment))
}

// An indented line of a comment is a godoc code block. Its digits are the thing
// being shown, and rewriting them to pass this rule would break the example.
func TestACodeBlockLineIsExempt(t *testing.T) {
	assert.Empty(t, Find("\tgo-toolchain --generate abc > log 2>&1", Comment))
	assert.Empty(t, Find("\tretries 3 times", Comment))
}

// The control beside it: the same sentence in the comment's own voice is read.
func TestAnUnindentedLineIsStillRead(t *testing.T) {
	assert.NotEmpty(t, Find("There are 3 retries.", Comment))
}

// Prose is a different substrate and keeps its own answer, so this cannot move
// a verdict on the merge gate.
func TestArityDoesNotReachProse(t *testing.T) {
	const line = "The repo's one plugin."
	assert.Empty(t, Find(line, Prose), "prose needs a plural noun anyway")
}
