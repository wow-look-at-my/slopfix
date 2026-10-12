package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Each sentence the sentence-length repair writes is a whole sentence under
// the cap. A gapped list of "X after Y" pairs is no sentence once divided from
// its verb, and a noun after a noun is no imperative clause.
func TestTheSentenceCapLeavesNoFragment(t *testing.T) {
	for in, fragment := range map[string]string{
		"This happens after `Open()`, a single read's body after the size guards, and a batch read's archive after every member is opened and checked against its stored digest.":  "A single read's body",
		"The rule reads each job whose runs-on value falls back to a hosted Linux label with no visibility test, and whether the job runs the docker client on its runner at all.": "Test, and whether",
	} {
		got := ste.FixSelected(in, func(id string) bool { return id == ste.IDSentenceCap })
		assert.NotContains(t, got, fragment, in)
		for _, sentence := range ste.Sentences(got) {
			sentence = strings.TrimSpace(sentence)
			assert.True(t, ste.StandsAlone(sentence), "%q is a fragment of %q", sentence, got)
			assert.LessOrEqual(t, ste.WordCount(sentence), ste.SentenceWordCap, "%q", sentence)
		}
	}
}

// A clause a subordinator opens is no main clause, so a gapped item with one
// does not stand alone.
func TestASubordinateClauseAloneIsNoSentence(t *testing.T) {
	assert.False(t, ste.StandsAlone("A single read's body after the size guards, and a batch read's archive after every member is opened."))
	assert.True(t, ste.StandsAlone("The slot goes back after every member is opened."))
}
