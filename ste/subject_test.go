package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// The clause after the colon opens on a plural noun the tagger can read as a
// verb, and its own verb follows the noun group.
func TestAPluralNounAfterAColonOpensItsOwnSubject(t *testing.T) {
	in := "The word is recalled after a tool result, not at a new user turn: models such as DeepSeek discard earlier thinking at every user turn by design, so a user-turn probe scores every host as dropping it."
	masked := mask(in)
	s := syntax.Parse(masked, opaque(in, masked))
	colon := -1
	for i, w := range s.Words {
		if w.Text == ":" {
			colon = i
		}
	}
	require.GreaterOrEqual(t, colon, 0)
	assert.True(t, pluralSubjectAt(s, colon+1), "%s\n%s", s.Tags(), s.Outline())
}
