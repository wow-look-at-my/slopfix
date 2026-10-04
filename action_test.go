package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// An input that narrows the run, such as a path or a rule list, lets a caller switch the check off.
func TestActionHasNoInputThatNarrowsTheCheck(t *testing.T) {
	content, err := os.ReadFile("action.yml")
	require.NoError(t, err)
	var action struct {
		Inputs map[string]any `yaml:"inputs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &action))

	var names []string
	for name := range action.Inputs {
		names = append(names, name)
	}
	slices.Sort(names)
	assert.Equal(t, []string{"level", "permission"}, names,
		"the action takes only the workflow-permission query; a new input must not scope or weaken the check")
}

// unixRunScript answers the run script of the action's Unix step.
func unixRunScript(t *testing.T) string {
	content, err := os.ReadFile("action.yml")
	require.NoError(t, err)
	var action struct {
		Runs struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &action))
	for _, step := range action.Runs.Steps {
		if step.ID == "unix" {
			return step.Run
		}
	}
	t.Fatal("action.yml has no unix step")
	return ""
}

// The findings of a check go to the log only. GitHub caps a step's outputs, and
// a large repository's findings in GITHUB_OUTPUT failed the step.
func TestTheActionKeepsCheckFindingsOutOfItsOutputs(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "slopfix")
	require.NoError(t, os.WriteFile(stub, []byte("echo \"finding for $*\"\n"), 0o755))
	script := strings.ReplaceAll(unixRunScript(t), "${{ steps.download.outputs.path }}", stub)

	for _, permission := range []string{"", "id-token"} {
		outputs := filepath.Join(dir, "outputs-"+permission)
		require.NoError(t, os.WriteFile(outputs, nil, 0o644))
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outputs, "SLOPFIX_PERMISSION="+permission, "SLOPFIX_LEVEL=write")
		log, err := cmd.CombinedOutput()
		require.NoError(t, err, string(log))
		assert.Contains(t, string(log), "finding for", "the log keeps what slopfix wrote")
		written, err := os.ReadFile(outputs)
		require.NoError(t, err)
		if permission == "" {
			assert.Empty(t, string(written), "a check writes no output")
			continue
		}
		assert.Contains(t, string(written), "stdout<<SLOPFIX_STDOUT_END\nfinding for check workflow-permission")
	}
}
