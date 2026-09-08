package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// samples carries text breaking each rule, so the claim can be driven rather
// than asserted.
var samples = map[string]string{
	ste.IDContraction: "It doesn't hold.",
	ste.IDModal:       "It should hold.",
	ste.IDSemicolon:   "It holds; it does not break.",
	ste.IDCommaSplice: "The loader reads the file, it returns the rows.",
	ste.IDSentenceCap: "The loader reads the file and returns the rows and checks the header and " +
		"reports the count and closes the handle and logs the answer and exits cleanly now.",
	ste.IDStaleCount: "There are three sections.",
}

// The assertion that keeps Repairs honest. A rule Fix rewrites must be in the
// set, and a rule it leaves alone must not be.
func TestRepairsNamesExactlyWhatFixRewrites(t *testing.T) {
	for id := range ste.AllIDs.All() {
		sample, ok := samples[id]
		require.True(t, ok, "no sample for %s", id)
		require.NotEmpty(t, ste.Check(sample, 1), "the sample for %s breaks no rule", id)

		rewritten := ste.FixSelected(sample, func(only string) bool { return only == id })
		if ste.Repairs.Contains(id) {
			assert.NotEqual(t, sample, rewritten, "%s is named as repairable and rewrote nothing", id)
			continue
		}
		assert.Equal(t, sample, rewritten, "%s rewrote text and is not named as repairable", id)
	}
}

// The negative control. A name that is not a rule is not in the set, so the
// case above passes on the answer rather than on a set saying yes to anything.
func TestANameThatIsNotARuleIsNotNamed(t *testing.T) {
	assert.False(t, ste.Repairs.Contains("ste/not-a-real-rule"))
	assert.True(t, ste.Repairs.Contains(ste.IDSemicolon))
}
