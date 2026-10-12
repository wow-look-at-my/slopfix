package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A cut before the dash that opens an aside once left the rest opening on the
// dash. No sentence opens there, so the division read as none.
func TestTheRestAfterACutOpensOnTheWordPastADash(t *testing.T) {
	for _, source := range []string{
		"reaches the real host worker — the shipped worker loop runs beside it",
		"reaches the real host worker – the shipped worker loop runs beside it",
	} {
		assert.Equal(t, "The shipped worker loop runs beside it", hardRest(source, len("reaches the real host worker")))
	}
	assert.Equal(t, "The loop runs", hardRest("a cut the loop runs", len("a cut")))
}
