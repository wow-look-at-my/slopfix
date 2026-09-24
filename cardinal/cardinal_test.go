package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// texts returns what a substrate reported.
func texts(text string, s Substrate) []string {
	var out []string
	for _, tok := range Find(text, s) {
		out = append(out, tok.Text)
	}
	return out
}

// A prose finding carries the quantity so the repair can cut the cardinal off
// the front of it, which a list of phrases does not show.
func TestAProseFindingOpensWithTheCardinal(t *testing.T) {
	found := Find("This repo's 15 plugins ride along.", Prose)
	require.Len(t, found, 1)
	assert.Equal(t, "15 plugins", found[0].Text)
	assert.Equal(t, Leading.FindString(found[0].Text), "15 ")
}

// The vocabularies differ, and the difference is deliberate. A comment reads
// the scales; prose does not, because there they are ordinary English.
func TestTheVocabulariesDifferByDesign(t *testing.T) {
	for _, word := range []string{"hundred", "thousand", "million"} {
		assert.True(t, commentWords.Contains(word), word)
		assert.False(t, proseWords.Contains(word), word)
	}
	for _, word := range []string{"three", "twenty", "dozen"} {
		assert.True(t, commentWords.Contains(word), word)
		assert.True(t, proseWords.Contains(word), word)
	}
	for _, word := range []string{"twenty", "dozen"} {
		assert.False(t, gateWords.Contains(word), word)
	}
}
