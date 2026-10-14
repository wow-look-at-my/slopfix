package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// One clause with a long tail of phrases: every cut leaves a rest with no
// verb. The last-resort division still divides, and gives the rest a
// subject and a verb. No path leaves the sentence over the cap.
func TestAHardDivisionWithNoVerbSafeCutCarriesTheRest(t *testing.T) {
	in := "The gate reads the cache file of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run."
	out, ok := hardDivision(in, checkMask(in), SentenceWordCap)
	assert.True(t, ok, out)
	assert.Zero(t, overCap(out, SentenceWordCap), out)
	assert.Regexp(t, `\. This holds [a-z]`, out)
}
