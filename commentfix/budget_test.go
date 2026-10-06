package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A block over the line cap is brought under it by rewriting: a blank marker
// line says nothing, so it goes before any prose does.
func TestAVolumeBlockGivesUpItsBlanksBeforeItsProse(t *testing.T) {
	text := []string{
		"// The loader reads the queue and writes each entry out to the caller.",
		"//",
		"// It waits for the next entry and then it repeats the same pass again.",
		"//",
		"// A read that finds the queue empty returns at once with no entry sent.",
	}
	out, ok := fitVolume(text, 3)
	require.True(t, ok, "the rewrite reaches the cap")
	assert.LessOrEqual(t, len(out), 3)
	for _, line := range text {
		if strings.TrimSpace(line) == "//" {
			continue
		}
		assert.Contains(t, out, line, "no prose line was dropped")
	}
}

// A structured block keeps its list lines whole: the rewrite never welds a
// list item to the prose around it.
func TestAVolumeRewriteKeepsListLinesWhole(t *testing.T) {
	text := []string{
		"// Create the handle.",
		"//",
		"// # Arguments",
		"// * `a` - the first thing",
		"// * `b` - the second thing",
	}
	out, ok := fitVolume(text, 4)
	require.True(t, ok, "the blank alone reaches the cap")
	assert.Contains(t, out, "// # Arguments")
	assert.Contains(t, out, "// * `a` - the first thing")
	assert.Contains(t, out, "// * `b` - the second thing")
}

// A block the rewrite cannot bring under the cap is reported, so the caller
// cuts rather than the budget silently dropping a thought.
func TestAVolumeBlockThatCannotFitIsReported(t *testing.T) {
	text := []string{
		"// The loader reads the queue and writes each entry out to the caller now.",
		"// It waits for the next entry and then it repeats the same pass once more.",
		"// A read that finds the queue empty returns at once with no entry sent.",
	}
	out, ok := fitVolume(text, 2)
	assert.False(t, ok, "three full prose lines carry more than two lines")
	assert.Equal(t, text, out, "nothing was dropped")
}
