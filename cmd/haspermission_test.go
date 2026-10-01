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

// askPermission runs the command in a work tree that holds one workflow, as a
// GitHub Actions step sees it.
func askPermission(t *testing.T, req permissionRequest) (permissionAnswer, error) {
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
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := hasPermission(cmd, req, func(k string) string { return env[k] })
	var answer permissionAnswer
	if out.Len() > 0 {
		require.NoError(t, json.Unmarshal(out.Bytes(), &answer))
	}
	return answer, err
}

func TestTheRunningJobIsReadFromTheEnvironment(t *testing.T) {
	answer, err := askPermission(t, permissionRequest{permission: "id-token", level: "write"})
	require.NoError(t, err)
	assert.True(t, answer.Granted)
	assert.Equal(t, "workflow", answer.Source)
}

func TestAssertFailsOnAMissingGrant(t *testing.T) {
	answer, err := askPermission(t, permissionRequest{permission: "id-token", level: "write", job: "tight"})
	require.NoError(t, err, "without --assert a missing grant is an answer, not a failure")
	assert.False(t, answer.Granted)

	answer, err = askPermission(t, permissionRequest{permission: "id-token", level: "write", job: "tight", assert: true})
	assert.ErrorContains(t, err, "is NOT granted")
	assert.False(t, answer.Granted, "the answer still reaches stdout")
}

func TestABadLevelIsAnError(t *testing.T) {
	_, err := askPermission(t, permissionRequest{permission: "id-token", level: "admin"})
	assert.ErrorContains(t, err, "not none, read or write")
}
