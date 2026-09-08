package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// texts returns what a substrate reported, which is what every case asserts on.
func texts(text string, s Substrate) []string {
	var out []string
	for _, tok := range Find(text, s) {
		out = append(out, tok.Text)
	}
	return out
}

// The frame is the whole difference between the substrates, so it opens the
// file. The same sentence is a finding in a comment and nothing in prose.
func TestTheFrameIsWhatSeparatesTheSubstrates(t *testing.T) {
	bare := "split it into three parts if that reads better"
	assert.Empty(t, texts(bare, Prose))
	assert.Equal(t, []string{"three"}, texts(bare, Comment))

	framed := "It ships three hooks."
	assert.Equal(t, []string{"three hooks"}, texts(framed, Prose))
}

// The merge gate reads a document with no frame, and pays for that with a list
// of units. The same sentence therefore parts both document substrates: a
// duration is measured for the gate and counted for the inventory rule.
func TestTheUnitsListIsTheGatesAloneAndTheFrameIsTheOtherSubstratesAlone(t *testing.T) {
	measured := "The read has 20 seconds."
	assert.Equal(t, []string{"20 seconds"}, texts(measured, Prose))
	assert.Empty(t, texts(measured, Gate))

	counted := "The read has 20 plugins."
	assert.Equal(t, []string{"20 plugins"}, texts(counted, Prose))
	assert.Equal(t, []string{"20 plugins"}, texts(counted, Gate))

	// No frame, so the gate reports what the inventory rule leaves alone.
	bare := "split it into three parts if that reads better"
	assert.Empty(t, texts(bare, Prose))
	assert.Equal(t, []string{"three parts"}, texts(bare, Gate))
}

// The gate's word list stops short of the prose one, and it reads any run of
// digits. Neither list is
// derived from the other, and widening either moves verdicts on the gate.
func TestTheGateVocabularyStopsWhereItAlwaysDid(t *testing.T) {
	assert.Empty(t, texts("it has twenty hooks", Gate))
	assert.Equal(t, []string{"twenty hooks"}, texts("It has twenty hooks.", Prose))

	assert.Equal(t, []string{"twelve hooks"}, texts("it has twelve hooks", Gate))
	assert.Equal(t, []string{"20000 hooks"}, texts("it has 20000 hooks", Gate))
	assert.Empty(t, texts("It has 20000 hooks.", Prose), "the prose rule caps the digits")
}

// Arithmetic is not a tally, and that guard is the gate's own.
func TestTheGateLeavesArithmeticAlone(t *testing.T) {
	assert.Empty(t, texts("a range of 3-4 items", Gate))
	assert.Equal(t, []string{"4 items"}, texts("it holds 4 items", Gate))
}

// Prose reports the quantity because the repair cuts the cardinal off the front
// of it. A comment reports the number, because there is nothing there to cut.
func TestEachSubstrateReportsWhatItsRepairNeeds(t *testing.T) {
	assert.Equal(t, []string{"15 plugins"}, texts("This repo's 15 plugins ride along.", Prose))
	assert.Equal(t, []string{"15"}, texts("the walk has 15 phases", Comment))

	found := Find("This repo's 15 plugins ride along.", Prose)
	require.Len(t, found, 1)
	assert.Equal(t, "15 plugins", found[0].Text)
	assert.Equal(t, Leading.FindString(found[0].Text), "15 ")
}

// The vocabularies differ, and the difference is deliberate. A comment reads
// the singular and the ordinals; prose reads neither, because there they are
// overwhelmingly ordinary English.
func TestTheVocabulariesDifferByDesign(t *testing.T) {
	for _, word := range []string{"one", "first", "hundred", "once"} {
		assert.True(t, commentWords.Contains(word), word)
		assert.False(t, proseWords.Contains(word), word)
	}
	for _, word := range []string{"three", "twenty", "dozen"} {
		assert.True(t, commentWords.Contains(word), word)
		assert.True(t, proseWords.Contains(word), word)
	}
	for _, word := range []string{"twenty", "dozen"} {
		assert.False(t, gateWords.Contains(word), word)
	}
}

// The exemptions belong to the substrate that has no frame to lean on.
func TestTheCommentExemptionsCarryTheShapesThatCountNothing(t *testing.T) {
	for name, text := range map[string]string{
		"status code":   "the proxy answers HTTP 403 here",
		"section":       "the rule in §7.3 governs this",
		"money":         "the run costs $1.43 of budget",
		"qualified":     "sync.Once guards it, and net/http serves it",
		"url":           "see https://example.com/v2/spec",
		"name digits":   "sha256 over the amd64 payload, capped at 10ms",
		"bound by dash": "KERN_PROCARGS2 opens with a 4-byte argc",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, texts(text, Comment))
		})
	}

	// The controls, so the cases above prove an exemption rather than a rule
	assert.Equal(t, []string{"403"}, texts("the proxy answers 403 here", Comment))
	assert.Equal(t, []string{"4"}, texts("the walk has 4 - phases", Comment))
}

// A quantity reached through a function word counts nothing, and a number that
// continues a longer number is a version rather than a tally.
func TestTheProseGuardsHoldInsideAFrame(t *testing.T) {
	assert.Empty(t, texts("it has 2 of the format drops", Prose))
	assert.Empty(t, texts("Version 2.1.205 clients keep the builtin.", Prose))
}
