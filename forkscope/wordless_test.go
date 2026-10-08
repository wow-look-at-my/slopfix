package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A row with no word matches any blank row of the base. That match says
// nothing about who wrote it, so it goes with the fork rows around it.
func TestAWordlessRowBetweenForkRowsIsTheForks(t *testing.T) {
	base := "//! Old head.\n//!\n//! Old tail.\nuse x;\n"
	fork := "//! New head.\n//!\n//! New tail.\nuse x;\n"
	s := Changed(base, fork)
	assert.True(t, s.Owns(1))
	assert.True(t, s.Owns(2), "the blank row sits between two rows the fork wrote")
	assert.True(t, s.Owns(3))
	assert.False(t, s.Owns(4), "a row with words that matches the base stays the base's")
}

// A wordless row beside a base row stays the base's.
func TestAWordlessRowBesideABaseRowIsTheBases(t *testing.T) {
	base := "//! Old head.\n//!\n//! Kept tail.\n"
	fork := "//! New head.\n//!\n//! Kept tail.\n"
	s := Changed(base, fork)
	assert.True(t, s.Owns(1))
	assert.False(t, s.Owns(2))
	assert.False(t, s.Owns(3))
}
