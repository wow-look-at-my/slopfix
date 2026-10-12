package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// samples carries text breaking each rule, so the claim can be driven rather
// than asserted. Every rule this package reports carries one.
var samples = map[string]string{
	ste.IDContraction: "It doesn't hold.",
	ste.IDModal:       "It should hold.",
	ste.IDSemicolon:   "It holds; it does not break.",
	ste.IDCommaSplice: "The loader reads the file, it returns the rows.",
	// Every coordinator here joins clauses that each name who acts, which is the
	// case the repair divides. A shared subject is the case it declines.
	ste.IDSentenceCap: "The loader reads the file and the caller waits for it and the header " +
		"check runs first and the count goes to the log and the handle closes at the end now.",
	ste.IDStaleCount:     "There are three sections.",
	ste.IDPostdeterminer: "Its four fields hold the header.",

	// The warning rules. Warn reports each rather than Check, and the repair of each is driven below the same way.
	ste.IDPassive:     "The file is read by the gate.",
	ste.IDTense:       "The gate has read the file.",
	ste.IDNounCluster: "The gate file system cache lookup stopped.",
	ste.IDDictionary:  "The tool gives additional output today.",
	ste.IDInstructionLength: "Read the file from the disk and write the result to the store " +
		"for the caller before the next build of the tree starts.",
	ste.IDParagraphLength: "The gate reads the file. The gate writes the result. " +
		"The gate waits for the caller. The gate stops the loop. The gate starts the build. " +
		"The gate ends the run. The gate fails the check.",
}

// The assertion that keeps Repairs honest. A rule Fix rewrites must be in the
// set, and a rule it leaves alone must not be.
func TestRepairsNamesExactlyWhatFixRewrites(t *testing.T) {
	for id := range ste.AllIDs.All() {
		sample, ok := samples[id]
		require.True(t, ok, "no sample for %s", id)
		require.NotEmpty(t, reported(id, sample), "the sample for %s breaks no rule", id)

		rewritten := ste.FixSelected(sample, func(only string) bool { return only == id })
		if ste.Repairs.Contains(id) {
			assert.NotEqual(t, sample, rewritten, "%s is named as repairable and rewrote nothing", id)
			assert.Empty(t, reported(id, rewritten), "%s still reports after its own repair: %s", id, rewritten)
			continue
		}
		assert.Equal(t, sample, rewritten, "%s rewrote text and is not named as repairable", id)
	}
}

// reported answers the findings one rule reports in text. Check answers an
// error rule, and Warn answers a warning, which Check never reports.
func reported(id, text string) []ste.Finding {
	var out []ste.Finding
	if ste.WarningIDs.Contains(id) {
		// A warning reads one paragraph, which is the unit the pipeline hands it:
		// a blank line divides the text into the paragraphs a reader sees.
		for _, para := range strings.Split(text, "\n\n") {
			for _, f := range ste.Warn(para, 1, false) {
				if f.ID == id {
					out = append(out, f)
				}
			}
		}
		return out
	}
	for _, f := range ste.Check(text, 1) {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}

// The negative control. A name that is not a rule is not in the set. So the
// case above passes on the answer, not on a set that says yes to anything.
func TestANameThatIsNotARuleIsNotNamed(t *testing.T) {
	assert.False(t, ste.Repairs.Contains("ste/not-a-real-rule"))
	assert.True(t, ste.Repairs.Contains(ste.IDSemicolon))
}

// The set is not empty, so the case above passes on rules rather than on an
// empty walk.
func TestTheRepairSetIsNotEmpty(t *testing.T) {
	require.NotEmpty(t, ste.Repairs.Len())
	assert.False(t, ste.Repairs.Contains(ste.IDStaleCount))
}
