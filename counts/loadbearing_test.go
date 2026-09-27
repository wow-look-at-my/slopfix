package counts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A number the sentence depends on is reported and never cut. Each case is a
// line a cut once made false.
func TestALoadBearingCountIsReportedNotCut(t *testing.T) {
	for _, doc := range []string{
		"The compiler has 7 phases:\n1. **Parsing** (`internal/syntax`) - lexer, parser, syntax tree\n2. **Type checking** (`internal/types2`) - type analysis\n",
		"Two methods opt a top-level test out.\n",
	} {
		t.Run(doc, func(t *testing.T) {
			assert.NotEmpty(t, Gate(doc), "the count is still reported")
			out, cut := StripGate(doc)
			assert.Equal(t, doc, out)
			assert.Empty(t, cut)
		})
	}
}

// The negative control: a bare tally still loses its number.
func TestATallyIsStillCut(t *testing.T) {
	out, cut := StripGate("It ships two hooks.")
	assert.Equal(t, "It ships hooks.", out)
	assert.NotEmpty(t, cut)
}

// A count after "the" goes too: "the arch payloads" already names the set.
func TestACountAfterTheIsCut(t *testing.T) {
	out, cut := StripGate("Shipping APEs: distribute release binaries zstd-compressed - the two arch payloads make APE images highly redundant, so the wire cost collapses.\n")
	assert.Equal(t, "Shipping APEs: distribute release binaries zstd-compressed - the arch payloads make APE images highly redundant, so the wire cost collapses.\n", out)
	assert.NotEmpty(t, cut)
}
