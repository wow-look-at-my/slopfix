package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A dictionary word inside a hyphenated name is part of the name. Rewriting
// it renames a plugin, a flag or a package.
func TestDictionaryLeavesAHyphenatedNameAlone(t *testing.T) {
	text := "The ask-properly plugin and the properly-named flag stay."
	for _, f := range Warn(text, 1, false) {
		assert.NotEqual(t, IDDictionary, f.ID, f.Detail)
	}
	assert.Equal(t, text, widenDictionary(text))
}

// A free-standing banned word still gets its approved word.
func TestDictionaryStillRepairsAFreeWord(t *testing.T) {
	assert.Equal(t, "It works correctly.", widenDictionary("It works properly."))
}
