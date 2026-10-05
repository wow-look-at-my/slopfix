package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// A sentence with no main clause divides into noun phrases, each under the cap
// and each closed with a stop.
func TestAFragmentDividesIntoFragments(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("a cache that never empties and never grows ", 6)) + "."
	out := ste.Fix(text)
	sentences := ste.Sentences(out)
	assert.Greater(t, len(sentences), 1, "the fragment divides: %q", out)
	for _, s := range sentences {
		assert.LessOrEqual(t, ste.WordCount(s), ste.SentenceWordCap, "%q", s)
		assert.True(t, strings.HasSuffix(strings.TrimSpace(s), "."), "%q ends on a stop", s)
	}
	assert.Equal(t, strings.Join(strings.Fields(strings.ToLower(text)), " "),
		strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(out, ".", ""))), " ")+".",
		"the division adds no word and drops none")
}

// A run that never closes keeps its leading noun phrase, closed with a stop.
func TestPhraseHeadKeepsTheLeadingPhrase(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("a clause that never closes and keeps going onward ", 8))
	head, ok := ste.PhraseHead(text, ste.SentenceWordCap)
	assert.True(t, ok)
	assert.True(t, strings.HasPrefix(head, "a clause that never closes and keeps going onward"), "%q", head)
	assert.True(t, strings.HasSuffix(head, "onward."), "%q", head)
	assert.LessOrEqual(t, ste.WordCount(head), ste.SentenceWordCap, "%q", head)

	_, ok = ste.PhraseHead("the cache reads the file and writes it back", ste.SentenceWordCap)
	assert.False(t, ok, "a run with no second noun phrase opener has no phrase head")
}

// DivideTo divides at a cap lower than the STE cap, and the first part keeps
// the opening words.
func TestDivideToKeepsTheOpeningWords(t *testing.T) {
	text := "The cache writes every entry to disk before it answers the caller, because a crash between the answer and the write loses the entry."
	out := ste.DivideTo(text, 15)
	first := ste.Sentences(out)[0]
	assert.LessOrEqual(t, ste.WordCount(first), 15, "%q", out)
	assert.True(t, strings.HasPrefix(first, "The cache writes every entry to disk"), "%q", out)
}
