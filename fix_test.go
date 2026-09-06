package slopfmt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfmt"
)

// prose puts a document to Fix with no path, which is how a caller vouches for
// text as prose.
func prose(content string) slopfmt.Repair {
	return slopfmt.Fix(slopfmt.Request{Content: content})
}

func TestFixJoinsAWrapAndCutsACount(t *testing.T) {
	repair := prose("There are three sections in the payload,\nand each one is read.\n")
	assert.True(t, repair.Changed)
	assert.Equal(t, "There are sections in the payload, and each one is read.\n", repair.Text)
	assert.Equal(t, []string{"three sections"}, repair.Removed)
}

func TestFixLeavesACleanDocumentAlone(t *testing.T) {
	doc := "A paragraph that needs no repair at all.\n"
	repair := prose(doc)
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Removed)
	assert.Empty(t, repair.Findings)
}

// A fence is data. Joining its lines changes what the code says.
func TestFixKeepsAFencedBlockWhole(t *testing.T) {
	doc := "# Title\n\n```sh\nfirst\nsecond\n```\n"
	repair := prose(doc)
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
}

// What a rewrite cannot repair is reported rather than guessed at.
func TestFixReportsWhatItCannotRepair(t *testing.T) {
	repair := prose("It doesn't expand its contraction.\n")
	require.NotEmpty(t, repair.Findings)
	assert.Contains(t, repair.Findings[0].Fix, "does not")
}

func TestFixReportsTheFindingsOfTheRepairedText(t *testing.T) {
	repair := prose("A sentence that is wrapped\nand that doesn't expand.\n")
	assert.True(t, repair.Changed)
	require.Len(t, repair.Findings, 1)
	assert.Equal(t, 1, repair.Findings[0].Line)
}

func TestOnlyTheNamedRuleRuns(t *testing.T) {
	doc := "There are three sections,\nand each is read.\n"
	repair := slopfmt.Fix(slopfmt.Request{Content: doc, Rules: []slopfmt.Rule{slopfmt.RuleCounts}})

	assert.Equal(t, []string{"three sections"}, repair.Removed)
	assert.Contains(t, repair.Text, "\nand each is read.")
	assert.Empty(t, repair.Findings)
}

func TestSourceGetsTheTombstoneRuleAndNoProseRule(t *testing.T) {
	src := "// This used to read the flag.\nx := \"a sentence far too long to pass the cap\"\n"
	repair := slopfmt.Fix(slopfmt.Request{Content: src, Path: "a.go"})

	assert.True(t, repair.Changed)
	assert.NotContains(t, repair.Text, "used to read")
	assert.Empty(t, repair.Findings)
}

func TestADocumentPathStillGetsTheProseRules(t *testing.T) {
	repair := slopfmt.Fix(slopfmt.Request{Content: "It doesn't expand.\n", Path: "a.md"})
	assert.NotEmpty(t, repair.Findings)
}
