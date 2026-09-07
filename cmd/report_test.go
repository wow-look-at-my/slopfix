package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// report drives the command and decodes what it wrote. The command is built per
// call, because tests run in parallel and rootCmd's writer is shared.
func report(t *testing.T, path string, only []string, content string) (reportOutput, string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader(content))
	err := reportContent(cmd, path, only)
	if err != nil {
		return reportOutput{}, out.String(), err
	}
	var decoded reportOutput
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	return decoded, out.String(), nil
}

func idsOf(out reportOutput) []string {
	ids := make([]string, 0, len(out.Findings))
	for _, finding := range out.Findings {
		ids = append(ids, finding.ID)
	}
	return ids
}

const workflowPath = ".github/workflows/ci.yml"

func TestAWorkflowIsReadByTheWorkflowRules(t *testing.T) {
	content := "name: CI\n# one\n# two\n# three\non:\n  push:\n    branches: ['**']\n"
	out, _, err := report(t, workflowPath, nil, content)
	require.NoError(t, err)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "yaml/comment-block", out.Findings[0].ID)
	assert.Equal(t, 2, out.Findings[0].Line)
	// The span is what an editor underlines, and the reason EndLine exists.
	assert.Equal(t, 4, out.Findings[0].EndLine)
	assert.Equal(t, workflowPath, out.Path)
}

// The negative control for the case above. A comment line inside the limit
// reports nothing, which is what proves the case can fail.
func TestASingleCommentLineIsNotAWall(t *testing.T) {
	out, _, err := report(t, workflowPath, nil, "name: CI\n# one\non:\n  push:\n")
	require.NoError(t, err)
	assert.Empty(t, out.Findings)
}

// A null here crashes a caller that reads the length of what came back.
func TestACleanFileAnswersWithAnEmptyList(t *testing.T) {
	_, raw, err := report(t, workflowPath, nil, "name: CI\non:\n  push:\n")
	require.NoError(t, err)
	assert.Contains(t, raw, `"findings":[]`)
}

func TestAMarkdownFileIsReadByTheProseRules(t *testing.T) {
	out, _, err := report(t, "docs/notes.md", nil, "This shouldn't run; it breaks two rules.\n")
	require.NoError(t, err)
	ids := idsOf(out)
	assert.Contains(t, ids, "ste/contraction")
	assert.Contains(t, ids, "ste/semicolon")
	for _, finding := range out.Findings {
		assert.Equal(t, 1, finding.Line)
		assert.Equal(t, finding.Line, finding.EndLine)
	}
}

func TestAHardWrappedParagraphIsReported(t *testing.T) {
	out, _, err := report(t, "docs/notes.md", nil, "A paragraph is one line.\nThis line continues it.\n")
	require.NoError(t, err)
	assert.Contains(t, idsOf(out), "wrap/hard-wrap")
}

func TestOnlyNarrowsTheReportToTheRuleNamed(t *testing.T) {
	content := "This shouldn't run; it breaks two rules.\n"
	all, _, err := report(t, "docs/notes.md", nil, content)
	require.NoError(t, err)
	require.Greater(t, len(all.Findings), 1)

	narrowed, _, err := report(t, "docs/notes.md", []string{"ste/semicolon"}, content)
	require.NoError(t, err)
	assert.Equal(t, []string{"ste/semicolon"}, idsOf(narrowed))
}

// A typo that selects nothing reads exactly like a clean file, so it is an
// error instead.
func TestAnUnknownRuleIsAnError(t *testing.T) {
	_, _, err := report(t, "docs/notes.md", []string{"ste/nosuch"}, "text\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown rule")
	assert.Contains(t, err.Error(), "ste/semicolon")
}

// The path decides which rules read the text, so there is no default for it.
func TestReportWithNoPathIsAnError(t *testing.T) {
	_, _, err := report(t, "", nil, "text\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--path is required")
}

// A finding is the answer rather than a failure. The caller decides what it
// means, which is why this command never exits non-zero over one.
func TestAFindingIsNotAnError(t *testing.T) {
	_, _, err := report(t, workflowPath, nil, "# one\n# two\non: push\n")
	require.NoError(t, err)
}
