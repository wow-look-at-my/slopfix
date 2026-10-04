package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// The incident: a comment block above a trigger took a pull request red, on a
// workflow whose author had read the rule.
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
	findings := workflow.Check("# first\n\n# second\non: {push: {branches: ['**']}}\n")

	require.Len(t, findings, 1)
	assert.Equal(t, "lines 1-3", findings[0].Detail)
}

func TestCodeBetweenCommentsStartsANewBlock(t *testing.T) {
	assert.Empty(t, checkOthers("# first\non: {push: {branches: ['**']}}\n# second\njobs: {}\n"))
}

func TestACarriageReturnDoesNotHideAComment(t *testing.T) {
	findings := workflow.Check("# one\r\n# two\r\non: {push: {branches: ['**']}}\r\n")

	require.Len(t, findings, 1)
	assert.Equal(t, workflow.IDCommentBlock, findings[0].ID)
}

func TestACommentBlockAtTheEndOfTheFileIsReported(t *testing.T) {
	findings := workflow.Check("on: {push: {branches: ['**']}}\n# one\n# two\n")

	require.Len(t, findings, 1)
	assert.Equal(t, 2, findings[0].Line)
}

// A # inside a block scalar opens a shell comment.
const scriptWithComments = "on: {push: {branches: ['**']}}\njobs:\n  build:\n    steps:\n      - run: |\n          # install the backend\n          # the suites need it\n          apt-get install -y bubblewrap\n          apt-get clean\n"

func TestShellCommentsInABlockScalarAreNotAYamlCommentBlock(t *testing.T) {
	assert.Empty(t, checkOthers(scriptWithComments))
}

func TestRepairingLeavesAScriptsCommentsAlone(t *testing.T) {
	repair := workflow.Fix(scriptWithComments, othersThanConcurrency)
	assert.False(t, repair.Changed)
	assert.Equal(t, scriptWithComments, repair.Text)
}

// Comment lines whose first never closed its sentence join into sentences, not
// one run-on. A line that wraps mid-sentence still joins as one.
func TestJoiningCommentLinesKeepsTheirSentencesApart(t *testing.T) {
	keep := func(string) bool { return true }
	for in, want := range map[string]string{
		"jobs:\n  build:\n    steps:\n      # No fetch-cacerts step: the CA bundle is a generate directive\n      # The org module proxy is gone, so modules come direct with sumdb off.\n      - run: make\n": "      # No fetch-cacerts step: the CA bundle is a generate directive. The org module proxy is gone, so modules come direct with sumdb off.\n",
		"jobs:\n  build:\n    steps:\n      # Stock Go cannot resolve the placeholder. Some are private, so\n      # the checksum database skips them.\n      - run: make\n":                                   "      # Stock Go cannot resolve the placeholder. Some are private, so the checksum database skips them.\n",
		"jobs:\n  build:\n    steps:\n      # The token is minted by\n      # GitHub for each run.\n      - run: make\n":                                                                                       "      # The token is minted by GitHub for each run.\n",
	} {
		repair := workflow.Fix(in, keep)
		require.True(t, repair.Changed, in)
		assert.Contains(t, repair.Text, want)
		assert.Empty(t, workflow.Check(repair.Text))
	}
}

// The scalar ends where the indentation does, so the comments after it are
// judged as the YAML comments they are.
func TestACommentBlockAfterABlockScalarIsStillReported(t *testing.T) {
	findings := checkOthers("on: {push: {branches: ['**']}}\njobs:\n  build:\n    steps:\n      - run: |\n          # a shell comment\n          make\n\n# one\n# two\n")

	require.Len(t, findings, 1)
	assert.Equal(t, "lines 9-10", findings[0].Detail)
}

// A folded scalar and the chomping and indentation indicators open a body too.
func TestEveryBlockScalarHeaderOpensABody(t *testing.T) {
	for _, header := range []string{"|", "|-", "|+", ">", ">-", ">+", "|2", "|2-"} {
		assert.Empty(t, workflow.Check("on: {push: {branches: ['**']}}\nscript: "+header+"\n  # one\n  # two\n"), "header %s", header)
	}
}
