package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// Each sentence came out of this tree over the cap, with the repair asking for a rewrite by hand.
var leftovers = []string{
	"It goes only where an EARLIER entry in the same list already sent stdout to /dev/null, so the reversed spelling, which sends stderr to the terminal, is left alone.",
	"Asking the model to re-emit the message instead costs a round trip, and the message it writes to comply puts the decision back into prose while explaining itself.",
	"readText numbers the lines a Read call returns, the way the Read tool does: the line number, a tab, and the line with a trailing carriage return cut.",
	"Every entry is grounded in this org's own written convention, and the rest are direct synonyms of an entry already here, kept narrow rather than speculative.",
}

func TestALeftoverSentenceDivides(t *testing.T) {
	for _, in := range leftovers {
		t.Run(in[:30], func(t *testing.T) {
			got := ste.Fix(in)
			for _, s := range ste.Sentences(got) {
				assert.LessOrEqual(t, ste.WordCount(s), ste.SentenceWordCap, "%q\n%s\n%s", got, syntax.Parse(in, nil).Tags(), syntax.Parse(in, nil).Outline())
				assert.True(t, ste.StandsAlone(s), "%q in %q", s, got)
			}
		})
	}
}
