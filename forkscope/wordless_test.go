package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A line with no word matches any blank line of the base. That match says
// nothing about who wrote it, so it goes with the fork lines around it.
func TestAWordlessLineBetweenForkLinesIsTheForks(t *testing.T) {
	base := "//! Old head.\n//!\n//! Old tail.\nuse x;\n"
	fork := "//! New head.\n//!\n//! New tail.\nuse x;\n"
	s := Changed(base, fork)
	assert.True(t, s.Owns(1))
	assert.True(t, s.Owns(2), "the blank line sits between two lines the fork wrote")
	assert.True(t, s.Owns(3))
	assert.False(t, s.Owns(4), "a line with words that matches the base stays the base's")
}

// A wordless line beside a base line stays the base's.
func TestAWordlessLineBesideABaseLineIsTheBases(t *testing.T) {
	base := "//! Old head.\n//!\n//! Kept tail.\n"
	fork := "//! New head.\n//!\n//! Kept tail.\n"
	s := Changed(base, fork)
	assert.True(t, s.Owns(1))
	assert.False(t, s.Owns(2))
	assert.False(t, s.Owns(3))
}
