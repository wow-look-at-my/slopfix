package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A line whose prose runs past a sentence end is cut back to that end.
func TestATrimmedLineStopsAtItsLastSentenceEnd(t *testing.T) {
	trimmed, ok := trimLineToSentenceEnd("// the loader reads the flag. it returns what it names")
	require.True(t, ok)

	assert.Equal(t, "// the loader reads the flag.", trimmed)
}

// A line with no sentence end has nothing to trim back to.
func TestATrimmedLineNeedsASentenceEnd(t *testing.T) {
	_, ok := trimLineToSentenceEnd("// the loader reads the flag")

	assert.False(t, ok)
}

// A block closer after the sentence end comes back with the trimmed line.
func TestATrimmedLineKeepsItsCloser(t *testing.T) {
	trimmed, ok := trimLineToSentenceEnd(" * the loader reads the flag. it returns what it names */")
	require.True(t, ok)

	assert.Equal(t, " * the loader reads the flag. */", trimmed)
}

// The trimmed body keeps the cap's lines and closes a block that was closed.
func TestATrimmedBodyKeepsTheCapAndTheCloser(t *testing.T) {
	body := []string{
		"/* the loader reads the flag and names it",
		" * the loader reads the flag and names it. it returns what it names",
		" * the loader reads the flag and names it. it returns what it names */",
	}

	kept, ok := trimBodyToSentenceEnd(body, 2)
	require.True(t, ok)
	require.Len(t, kept, 2)

	assert.Equal(t, "/* the loader reads the flag and names it", kept[0])
	assert.True(t, strings.HasSuffix(kept[1], "names it. */"), kept[1])
}

// A body already within the cap is the caller's to leave alone.
func TestATrimmedBodyLeavesABodyWithinTheCap(t *testing.T) {
	body := []string{"// the loader reads the flag", "// it returns what it names"}

	_, ok := trimBodyToSentenceEnd(body, 3)

	assert.False(t, ok)
}
