package slopfix_test

import _ "embed"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

//go:embed testdata/workflow-with-findings.yml
var workflowWithFindings string

func TestCheckFileSendsAWorkflowToTheWorkflowRules(t *testing.T) {
	path := writeFile(t, ".github/workflows/ci.yml", workflowWithFindings)

	findings, err := slopfix.CheckFile(path)
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

	findings, err := slopfix.CheckFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, findings)
}

func TestCheckFileStillReadsADocumentWithTheProseRules(t *testing.T) {
	path := writeFile(t, "notes.md", testCheckFileStillReadsADocumentWithTheProseRulesMd)

	findings, err := slopfix.CheckFile(path)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, slopfix.IDHardWrap, findings[0].ID)
}

// A newline in YAML is syntax. Joining a wrapped concurrency: block makes
// GitHub reject the whole workflow before a job starts.
func TestFixFileNeverJoinsAWorkflow(t *testing.T) {
	content := testFixFileNeverJoinsAWorkflowYml
	path := writeFile(t, ".github/workflows/ci.yml", content)

	_, err := slopfix.FixFile(path)
	require.NoError(t, err)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, content, string(after))
}

//go:embed testdata/test-check-file-still-reads-a-document-with-the-prose-rules.md
var testCheckFileStillReadsADocumentWithTheProseRulesMd string

//go:embed testdata/test-fix-file-never-joins-a-workflow.yml
var testFixFileNeverJoinsAWorkflowYml string
