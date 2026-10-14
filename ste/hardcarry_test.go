package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/syntax"
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

// A sentence with no verb at all: no head reads as a sentence, so the last
// resort cuts after a word that closes a phrase. The head and the rest each
// take a carrier, and both read as sentences under the cap.
func TestTheLastResortDivisionCarriesBothHalves(t *testing.T) {
	in := "A list of the cache files of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run."
	masked := checkMask(in)
	out, ok := lastResortDivision(in, masked, syntax.Parse(masked, nil), SentenceWordCap)
	assert.True(t, ok, out)
	assert.Equal(t, "This covers a list of the cache files of the build host for each module of the tree. "+
		"This holds in the order of the paths of the list of the repository at the start of the run.", out)
	assert.Zero(t, overCap(out, SentenceWordCap), out)
}
