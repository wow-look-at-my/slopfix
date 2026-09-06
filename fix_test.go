package slopfmt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfmt"
)

<<<<<<< HEAD
// prose puts a document to Fix with no path, which is how a caller vouches for
// text as prose.
func prose(content string) slopfmt.Repair {
	return slopfmt.Fix(slopfmt.Request{Content: content})
}

func TestFixJoinsAWrapAndCutsACount(t *testing.T) {
	repair := prose("There are three sections in the payload,\nand each one is read.\n")
=======
func TestFixJoinsAWrapAndCutsACount(t *testing.T) {
	repair := slopfmt.Fix("There are three sections in the payload,\nand each one is read.\n")
>>>>>>> origin/master
	assert.True(t, repair.Changed)
	assert.Equal(t, "There are sections in the payload, and each one is read.\n", repair.Text)
	assert.Equal(t, []string{"three sections"}, repair.Removed)
}

func TestFixLeavesACleanDocumentAlone(t *testing.T) {
	doc := "A paragraph that needs no repair at all.\n"
<<<<<<< HEAD
	repair := prose(doc)
=======
	repair := slopfmt.Fix(doc)
>>>>>>> origin/master
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Removed)
	assert.Empty(t, repair.Findings)
}

// A fence is data. Joining its lines changes what the code says.
func TestFixKeepsAFencedBlockWhole(t *testing.T) {
	doc := "# Title\n\n```sh\nfirst\nsecond\n```\n"
<<<<<<< HEAD
	repair := prose(doc)
=======
	repair := slopfmt.Fix(doc)
>>>>>>> origin/master
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
}

// What a rewrite cannot repair is reported rather than guessed at.
func TestFixReportsWhatItCannotRepair(t *testing.T) {
<<<<<<< HEAD
	repair := prose("It doesn't expand its contraction.\n")
=======
	repair := slopfmt.Fix("It doesn't expand its contraction.\n")
>>>>>>> origin/master
	require.NotEmpty(t, repair.Findings)
	assert.Contains(t, repair.Findings[0].Fix, "does not")
}

func TestFixReportsTheFindingsOfTheRepairedText(t *testing.T) {
<<<<<<< HEAD
	repair := prose("A sentence that is wrapped\nand that doesn't expand.\n")
=======
	repair := slopfmt.Fix("A sentence that is wrapped\nand that doesn't expand.\n")
>>>>>>> origin/master
	assert.True(t, repair.Changed)
	require.Len(t, repair.Findings, 1)
	assert.Equal(t, 1, repair.Findings[0].Line)
}
<<<<<<< HEAD

func TestOnlyOneRuleRunsWhenACallerNamesOne(t *testing.T) {
	doc := "There are three sections,\nand each one is read.\n"
	repair := slopfmt.Fix(slopfmt.Request{Content: doc, Rules: []slopfmt.Rule{slopfmt.RuleCounts}})

	assert.Equal(t, []string{"three sections"}, repair.Removed)
	assert.Contains(t, repair.Text, "\nand each one is read.")
	assert.Empty(t, repair.Findings)
}

func TestSourceGetsTheTombstoneRuleAndNoProseRule(t *testing.T) {
	src := "// This used to read the flag.\nx := \"a sentence that is far too long to pass\"\n"
	repair := slopfmt.Fix(slopfmt.Request{Content: src, Path: "a.go"})

	assert.True(t, repair.Changed)
	assert.NotContains(t, repair.Text, "used to read")
	assert.Empty(t, repair.Findings)
}

func TestADocumentPathStillGetsTheProseRules(t *testing.T) {
	repair := slopfmt.Fix(slopfmt.Request{Content: "It doesn't expand.\n", Path: "a.md"})
	assert.NotEmpty(t, repair.Findings)
}
=======
>>>>>>> origin/master
