package blamelanguage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A marker over the whole sentence says something is wrong without saying which
// words, so the reader has nothing to act on. The finding names the wording.
func TestTheHitNamesThePhrase(t *testing.T) {
	hits := Check("The suite is red, but the failure is pre-existing and I left it.")
	require.Len(t, hits, 1)
	assert.Equal(t, "pre-existing", hits[0].Phrase)
	assert.Contains(t, hits[0].Sentence, "The suite is red", "the line stays for context")
}

// The phrase comes back as the writer spelled it, not as the table spells it.
func TestThePhraseKeepsTheWritersSpelling(t *testing.T) {
	hits := Check("This is PRE-EXISTING.")
	require.Len(t, hits, 1)
	assert.Equal(t, "PRE-EXISTING", hits[0].Phrase)
}

// A phrase split by a line wrap comes back whole, with the wrap collapsed, so
// the report reads as the writer meant it.
func TestAWrappedPhraseIsNamedWhole(t *testing.T) {
	hits := Check("The suite is green.\nIt is worth your\nattention though.")
	require.Len(t, hits, 1)
	assert.Equal(t, "worth your attention", hits[0].Phrase)
}

// Every row in the table can name itself, so no row reports a phrase it did not
// match.
func TestEveryPhraseNamesItself(t *testing.T) {
	for _, phrase := range phrases {
		hits := Check("The build is red. " + phrase + " so here we are.")
		require.NotEmpty(t, hits, "no hit for %q", phrase)
		assert.Equal(t, phrase, strings.ToLower(hits[0].Phrase))
	}
}

// A phrase at the very end of the message has no offset after it, and the
// original's tail is the answer there.
func TestAPhraseAtTheEndIsNamed(t *testing.T) {
	hits := Check("I am leaving this one: out of scope")
	require.Len(t, hits, 1)
	assert.Equal(t, "out of scope", hits[0].Phrase)
}
