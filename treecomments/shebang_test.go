package treecomments

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A grammar reads the interpreter line as a comment, because it opens on the
// same marker. Nothing downstream may see it: a rule that reflows a run would
// weld the prose under it onto the interpreter. The kernel then has no
// program to start.
func TestTheShebangIsNotAComment(t *testing.T) {
	src := testTheShebangIsNotACommentSh

	got := Extract("run.sh", src)
	require.Len(t, got, 1)
	assert.Equal(t, "# what this script does", got[0].Text)
	assert.Equal(t, 2, got[0].Line)
}

// The run beneath a shebang stands on its own, so a reflow rewrites it without
// reaching the line above.
func TestAShebangStartsNoRun(t *testing.T) {
	src := testAShebangStartsNoRunSh

	runs := Runs("run.sh", src)
	require.Len(t, runs, 1)
	assert.Len(t, runs[0], 2)
	assert.Equal(t, 2, runs[0][0].Line)
}

// A `#!` that is not the top line at the left edge is prose somebody wrote.
func TestAMarkerBelowTheTopLineStaysAComment(t *testing.T) {
	src := "# a note\n# on `#!` lines\nexit 0\n"

	assert.Len(t, Extract("run.sh", src), 2)
}

//go:embed testdata/test-the-shebang-is-not-a-comment.sh
var testTheShebangIsNotACommentSh string

//go:embed testdata/test-a-shebang-starts-no-run.sh
var testAShebangStartsNoRunSh string
