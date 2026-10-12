package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// askPermission runs check --permission in a work tree that holds one workflow,
// as a GitHub Actions step sees it.
func askPermission(t *testing.T, req permissionRequest) (permissionAnswer, string, error) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := "permissions:\n  id-token: write\njobs:\n  build:\n    steps: []\n  tight:\n    permissions:\n      contents: read\n    steps: []\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci.yml"), []byte(body), 0o644))
	env := map[string]string{
		"GITHUB_WORKFLOW_REF": "owner/repo/.github/workflows/ci.yml@refs/heads/main",
		"GITHUB_WORKSPACE":    root,
		"GITHUB_JOB":          "build",
	}
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := checkPermission(cmd, req, func(k string) string { return env[k] })
	var answer permissionAnswer
	if out.Len() > 0 {
		require.NoError(t, json.Unmarshal(out.Bytes(), &answer))
	}
	return answer, errOut.String(), err
}

func TestTheRunningJobIsReadFromTheEnvironment(t *testing.T) {
	answer, _, err := askPermission(t, permissionRequest{permission: "id-token", level: "write", asJSON: true})
	require.NoError(t, err)
	assert.True(t, answer.Granted)
	assert.Equal(t, "workflow", answer.Source)
}

// A missing grant is a finding: exit 1 as check, an answer under --json.
func TestAMissingGrantIsAFinding(t *testing.T) {
	_, said, err := askPermission(t, permissionRequest{permission: "id-token", level: "write", job: "tight"})
	assert.ErrorIs(t, err, errFindings)
	assert.Contains(t, said, "is NOT granted")

	answer, _, err := askPermission(t, permissionRequest{permission: "id-token", level: "write", job: "tight", asJSON: true})
	require.NoError(t, err, "under --json the caller decides what a finding means")
	assert.False(t, answer.Granted)
}

// check routes the name to the subcommand, and a file argument to check itself.
func TestWorkflowPermissionIsACheckSubcommand(t *testing.T) {
	found, rest, err := checkCmd.Find([]string{"workflow-permission", ".github/workflows/ci.yml"})
	require.NoError(t, err)
	assert.Equal(t, "workflow-permission", found.Name())
	assert.Equal(t, []string{".github/workflows/ci.yml"}, rest)

	found, _, err = checkCmd.Find([]string{"docs/a.md"})
	require.NoError(t, err)
	assert.Equal(t, "check", found.Name())
}

func TestABadLevelIsAnError(t *testing.T) {
	_, _, err := askPermission(t, permissionRequest{permission: "id-token", level: "admin"})
	assert.ErrorContains(t, err, "not none, read or write")
}
