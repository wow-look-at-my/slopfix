package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A sentence that a division opens with an identifier keeps the identifier's
// case. A capital turns foreignJobs into a name that nothing defines.
func TestAnOpeningIdentifierKeepsItsCase(t *testing.T) {
	for _, name := range []string{"foreignJobs also answers", "max_lines holds", "utf8 decodes", "x86 runs"} {
		assert.True(t, NameLike(name), name)
		assert.Equal(t, name, capitalizeOpening(name))
		assert.Equal(t, name, capitalize(name))
	}
	for _, word := range []string{"the gate reads", "it holds", "also, it runs"} {
		assert.False(t, NameLike(word), word)
		assert.NotEqual(t, word, capitalizeOpening(word))
	}
	assert.Equal(t, "`foreignJobs` answers", capitalizeOpening("`foreignJobs` answers"))
}
