package workflow_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// The incident: a comment block above a trigger took a pull request red, on a
// workflow whose author had read the rule.
func TestACommentRunPastTheLimitIsReported(t *testing.T) {
	findings := workflow.Check(testACommentRunPastTheLimitIsReportedYml)

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
	findings := workflow.Check(testABlankLineDoesNotSplitACommentBlockMd)

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
	findings := workflow.Check(testACommentBlockAtTheEndOfTheFileIsReportedYml)

	require.Len(t, findings, 1)
	assert.Equal(t, 2, findings[0].Line)
}

//go:embed testdata/script-with-comments.yml
var scriptWithComments string

func TestShellCommentsInABlockScalarAreNotAYamlCommentBlock(t *testing.T) {
	assert.Empty(t, checkOthers(scriptWithComments))
}

func TestRepairingLeavesAScriptsCommentsAlone(t *testing.T) {
	repair := workflow.Fix(scriptWithComments, othersThanConcurrency)
	assert.False(t, repair.Changed)
	assert.Equal(t, scriptWithComments, repair.Text)
}

// A comment repair may change how many lines a script comment takes, and any
// change to the script's code is still refused.
func TestAScriptCommentMayShrinkButItsCodeMayNot(t *testing.T) {
	gate := workflow.Options(fixer.Options{}).Comments
	shorter := edit.Lines(scriptWithComments, 5, 6, 0, []string{"          # the suites need the backend"})
	res := gate(scriptWithComments, []edit.Edit{shorter}, edit.Scope{})
	require.Len(t, res.Applied, 1)
	assert.Equal(t, testAScriptCommentMayShrinkButItsCodeMayNotYml, res.Text)

	code := edit.Lines(scriptWithComments, 5, 7, 0, []string{"          # the suites need the backend"})
	res = gate(scriptWithComments, []edit.Edit{code}, edit.Scope{})
	assert.Empty(t, res.Applied)
	assert.Len(t, res.Refused, 1)
	assert.Equal(t, scriptWithComments, res.Text)
}

// The interpreter line and a # line inside a heredoc are code and data. A
// comment edit that removes either is refused.
func TestAShebangAndAHeredocLineAreNotComments(t *testing.T) {
	gate := workflow.Options(fixer.Options{}).Comments
	head := testAShebangAndAHeredocLineAreNotCommentsYml
	for name, script := range map[string]string{
		"shebang": head + testAShebangAndAHeredocLineAreNotCommentsSh,
		"heredoc": head + "          cat <<EOF\n          # not a comment\n          EOF\n",
	} {
		res := gate(script, []edit.Edit{edit.Lines(script, 5, 5, 0, nil)}, edit.Scope{})
		if name == "heredoc" {
			res = gate(script, []edit.Edit{edit.Lines(script, 6, 6, 0, nil)}, edit.Scope{})
		}
		assert.Empty(t, res.Applied, name)
		assert.Len(t, res.Refused, 1, name)
		assert.Equal(t, script, res.Text, name)
	}
}

// Comment lines whose first never closed its sentence join into sentences, not
// one run-on. A line that wraps mid-sentence still joins as one.
func TestJoiningCommentLinesKeepsTheirSentencesApart(t *testing.T) {
	keep := func(string) bool { return true }
	for in, want := range map[string]string{
		testJoiningCommentLinesKeepsTheirSentencesApartYml:  "      # No fetch-cacerts step: the CA bundle is a generate directive. The org module proxy is gone, so modules come direct with sumdb off.\n",
		testJoiningCommentLinesKeepsTheirSentencesApartYml2: "      # Stock Go cannot resolve the placeholder. Some are private, so the checksum database skips them.\n",
		testJoiningCommentLinesKeepsTheirSentencesApartYml3: "      # The token is minted by GitHub for each run.\n",
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
	findings := checkOthers(testACommentBlockAfterABlockScalarIsStillReportedYml)

	require.Len(t, findings, 1)
	assert.Equal(t, "lines 9-10", findings[0].Detail)
}

// A folded scalar and the chomping and indentation indicators open a body too.
func TestEveryBlockScalarHeaderOpensABody(t *testing.T) {
	for _, header := range []string{"|", "|-", "|+", ">", ">-", ">+", "|2", "|2-"} {
		assert.Empty(t, workflow.Check("on: {push: {branches: ['**']}}\nscript: "+header+"\n  # one\n  # two\n"), "header %s", header)
	}
}

//go:embed testdata/test-a-comment-run-past-the-limit-is-reported.yml
var testACommentRunPastTheLimitIsReportedYml string

//go:embed testdata/test-a-blank-line-does-not-split-a-comment-block.md
var testABlankLineDoesNotSplitACommentBlockMd string

//go:embed testdata/test-a-comment-block-after-a-block-scalar-is-still-reported.yml
var testACommentBlockAfterABlockScalarIsStillReportedYml string

//go:embed testdata/test-a-comment-block-at-the-end-of-the-file-is-reported.yml
var testACommentBlockAtTheEndOfTheFileIsReportedYml string

//go:embed testdata/test-joining-comment-lines-keeps-their-sentences-apart.yml
var testJoiningCommentLinesKeepsTheirSentencesApartYml string

//go:embed testdata/test-joining-comment-lines-keeps-their-sentences-apart-2.yml
var testJoiningCommentLinesKeepsTheirSentencesApartYml2 string

//go:embed testdata/test-joining-comment-lines-keeps-their-sentences-apart-3.yml
var testJoiningCommentLinesKeepsTheirSentencesApartYml3 string

//go:embed testdata/test-a-script-comment-may-shrink-but-its-code-may-not.yml
var testAScriptCommentMayShrinkButItsCodeMayNotYml string

//go:embed testdata/test-a-shebang-and-a-heredoc-line-are-not-comments.yml
var testAShebangAndAHeredocLineAreNotCommentsYml string

//go:embed testdata/test-a-shebang-and-a-heredoc-line-are-not-comments.sh
var testAShebangAndAHeredocLineAreNotCommentsSh string
