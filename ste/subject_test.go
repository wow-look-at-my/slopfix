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
	in := "The schema is checked after a migration, not at a new connection: models such as Prisma cache earlier schemas at every connection by design, so a connection-only probe scores every server as stale."
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
