package slopfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfmt"
	"github.com/wow-look-at-my/slopfmt/workflow"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

const workflowWithFindings = "on: push\n\n# one\n# two\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n"

func TestCheckFileSendsAWorkflowToTheWorkflowRules(t *testing.T) {
	path := writeFile(t, ".github/workflows/ci.yml", workflowWithFindings)

	findings, err := slopfmt.CheckFile(path)
	require.NoError(t, err)

	var ids []string
	for _, finding := range findings {
		ids = append(ids, finding.ID)
	}
	assert.Contains(t, ids, workflow.IDCommentBlock)
	assert.Contains(t, ids, workflow.IDAllBuildsJob)
}

// A workflow named from inside its own directory is still a workflow.
func TestCheckFileSniffsAWorkflowThePathDoesNotName(t *testing.T) {
	path := writeFile(t, "ci.yml", workflowWithFindings)

	findings, err := slopfmt.CheckFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, findings)
}

func TestCheckFileStillReadsADocumentWithTheProseRules(t *testing.T) {
	path := writeFile(t, "notes.md", "# Title\n\nA paragraph the author wrapped\nacross two lines by hand.\n")

	findings, err := slopfmt.CheckFile(path)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, slopfmt.IDHardWrap, findings[0].ID)
}

// A newline in YAML is syntax. Joining a wrapped concurrency: block makes
// GitHub reject the whole workflow before a job starts.
func TestFormatFileRefusesAWorkflow(t *testing.T) {
	content := "on: push\nconcurrency:\n  group: release\n"
	path := writeFile(t, ".github/workflows/ci.yml", content)

	_, err := slopfmt.FormatFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newlines are syntax")

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, content, string(after))
}
