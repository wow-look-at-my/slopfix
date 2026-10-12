package english

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A filler word that is a word of a proper name stays, because the name is
// what the sentence refers to.
func TestADropLeavesAProperNameWhole(t *testing.T) {
	for _, surface := range []string{Document, Comment} {
		assert.Equal(t, "An Actually Portable Executable is such a binary.",
			Fix("An Actually Portable Executable is such a binary.", surface))
		assert.Equal(t, "It fails.", Fix("It actually fails.", surface))
	}
}
