package counts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func phrases(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Phrase)
	}
	return out
}

func TestEachFrameReportsItsCount(t *testing.T) {
	for _, doc := range []string{
		"This repo's 15 plugins ride in the payload.",
		"It ships two hooks.",
		"There are three sections.",
		"The four rules below decide it.",
	} {
		assert.NotEmpty(t, Check(doc), doc)
	}
}

func TestAQuantityWithoutAFrameIsLeftAlone(t *testing.T) {
	for _, doc := range []string{
		"Every plugin this repo installs rides in the payload.",
		"Split it into three parts if that reads better.",
		"Version 2.1.205 clients keep the builtin.",
	} {
		assert.Empty(t, Check(doc), doc)
	}
}

// A limit and a size rot exactly as a tally does. A budget gets raised and a
// suite gets slower, and the document keeps asserting the old value.
func TestAMeasurementIsACount(t *testing.T) {
	assert.NotEmpty(t, Check("The read has 20 seconds."))
	assert.NotEmpty(t, Check("It carries 500 lines."))
	// A doc recorded the range a build measured, and the range moved.
	assert.NotEmpty(t, Check("An unchanged second build measures 60 seconds."))
	assert.NotEmpty(t, Check("The step takes about 90 seconds."))
	assert.Empty(t, Check("The budget lives in ci.yml, which is where to read it."))
}

func TestAFunctionWordBreaksTheCount(t *testing.T) {
	assert.Empty(t, Check("it has 2 of the format drops"))
}

// The exemptions come from the markdown splitter, so a fence, a table, a
// heading and an indented block are all data.
func TestVerbatimBlocksAreNeverJudged(t *testing.T) {
	assert.Empty(t, Check("```go\n// it has three sections\n```"))
	assert.Empty(t, Check("| it has three sections |\n|---|"))
	assert.Empty(t, Check("# it has three sections"))
	assert.Empty(t, Check("    it has three sections"))
}

func TestAnInlineCodeSpanIsData(t *testing.T) {
	assert.Empty(t, Check("Write `there are three sections` instead."))
}

func TestStripCutsTheNumberAndKeepsTheRest(t *testing.T) {
	out, cut := Strip("This repo's 15 plugins ride in the payload.")
	assert.Equal(t, "This repo's plugins ride in the payload.", out)
	assert.Equal(t, []string{"15 plugins"}, phrases(cut))
}

// Back to front, so an earlier span's offsets stay valid.
func TestStripHandlesSeveralCountsInOneDocument(t *testing.T) {
	out, cut := Strip("It ships two hooks.\n\nThere are three sections.\n")
	assert.Equal(t, "It ships hooks.\n\nThere are sections.\n", out)
	require.Len(t, cut, 2)
}

func TestStripLeavesACleanDocumentUntouched(t *testing.T) {
	doc := "Every plugin this repo installs rides in the payload.\n"
	out, cut := Strip(doc)
	assert.Equal(t, doc, out)
	assert.Empty(t, cut)
}

func TestAHitNamesItsLine(t *testing.T) {
	hits := Check("Intro line.\n\nThere are three sections.\n")
	require.Len(t, hits, 1)
	assert.Equal(t, "There are three sections.", hits[0].Line)
	assert.Equal(t, 3, hits[0].LineNo)
}
