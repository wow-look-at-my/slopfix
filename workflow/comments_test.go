package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfmt/workflow"
)

// The incident: a comment block above a trigger took a pull request red, on a
// workflow whose author had just read the rule.
func TestACommentRunPastTheLimitIsReported(t *testing.T) {
	findings := workflow.Check("name: CI\n\n# A preview is republished by pushing, so a branch whose\n# last run predates a change had no way to pick it up.\n# This trigger exists for that.\non:\n  workflow_dispatch:\n")

	require.Len(t, findings, 1)
	assert.Equal(t, workflow.IDCommentBlock, findings[0].ID)
	assert.Equal(t, 3, findings[0].Line)
	assert.Contains(t, findings[0].Rule, "3 comment lines in a row")
	assert.Equal(t, "lines 3-5", findings[0].Detail)
}

func TestOneCommentLineIsAllowed(t *testing.T) {
	assert.Empty(t, workflow.Check("# Republish a preview without a commit that only triggers one.\non:\n  workflow_dispatch:\n"))
}

// A blank line does not end a block. A reader sees a single paragraph.
func TestABlankLineDoesNotSplitACommentBlock(t *testing.T) {
	findings := workflow.Check("# first\n\n# second\non: push\n")

	require.Len(t, findings, 1)
	assert.Equal(t, "lines 1-3", findings[0].Detail)
}

func TestCodeBetweenCommentsStartsANewBlock(t *testing.T) {
	assert.Empty(t, workflow.Check("# first\non: push\n# second\njobs: {}\n"))
}

func TestACarriageReturnDoesNotHideAComment(t *testing.T) {
	findings := workflow.Check("# one\r\n# two\r\non: push\r\n")

	require.Len(t, findings, 1)
	assert.Equal(t, workflow.IDCommentBlock, findings[0].ID)
}

func TestACommentBlockAtTheEndOfTheFileIsReported(t *testing.T) {
	findings := workflow.Check("on: push\n# one\n# two\n")

	require.Len(t, findings, 1)
	assert.Equal(t, 2, findings[0].Line)
}
