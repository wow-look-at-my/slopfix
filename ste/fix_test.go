package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// A repair that leaves its own finding standing loops the caller forever.
func TestFixClearsTheMechanicalFindings(t *testing.T) {
	text := "It doesn't matter; a caller should wait, so the write fails."
	assert.NotEmpty(t, ste.Check(text, 1))
	assert.Empty(t, ste.Check(ste.Fix(text), 1))
}
