package tombstones

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A token inside a URL is no code name. A name outside it still is.
func TestATokenInsideAURLIsNoCandidate(t *testing.T) {
	words := identifierWords(withoutURLs("See https://claude.ai/code/session_01ExampleSessionIdentifier for the run of parseRunnerLabel."))
	assert.NotContains(t, words, "session_01ExampleSessionIdentifier")
	assert.Contains(t, words, "parseRunnerLabel")
}
