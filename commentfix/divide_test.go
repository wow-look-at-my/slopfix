package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Lines that end thoughts with no stop divide into sentences, so a cut can land on one of their ends.
func TestDivideProseClosesEachThought(t *testing.T) {
	text := []string{"// The loop reads the input", "// The loop reads the header", "// The loop reads the body"}
	out, ok := divideProse(text)
	require.True(t, ok)
	assert.Equal(t, []string{"// The loop reads the input. The loop reads the header. The loop reads the", "// body."}, out)
}

// A heading keeps its own line, and the prose under it divides.
func TestDivideProseKeepsAHeading(t *testing.T) {
	text := []string{"/// # Safety", "/// The caller keeps the pointer valid", "/// The pool reads it"}
	out, ok := divideProse(text)
	require.True(t, ok)
	assert.Equal(t, "/// # Safety", out[0])
	assert.Equal(t, "/// The caller keeps the pointer valid. The pool reads it.", strings.Join(out[1:], " "))
}

// A run of aligned columns is data, so no division touches it.
func TestDivideProseLeavesAnAlignedRun(t *testing.T) {
	text := []string{"//   input    the loop reads it", "//   header   the loop reads it"}
	out, ok := divideProse(text)
	assert.False(t, ok)
	assert.Equal(t, text, out)
	assert.Equal(t, 0, capCount(text))
}
