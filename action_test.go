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
	assert.Empty(t, names, "the action takes no input; an input must not scope or weaken the check")
}

type actionStep struct {
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
}

// An ubuntu runner answers a direct exec of the APE with "unable to find an
// interpreter", so the check on Unix goes through sh.
func TestActionRunsTheBinaryThroughShOnUnix(t *testing.T) {
	content, err := os.ReadFile("action.yml")
	require.NoError(t, err)
	var action struct {
		Runs struct {
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &action))
	idx := slices.IndexFunc(action.Runs.Steps, func(s actionStep) bool { return s.ID == "check" })
	require.GreaterOrEqual(t, idx, 0)
	assert.Contains(t, action.Runs.Steps[idx].Run, `sh "${{ steps.download.outputs.path }}" check .`)
}

// Several org actions call this in the same run. The claim keeps the check to
// one job of the run.
func TestActionClaimsTheCheckOncePerRun(t *testing.T) {
	content, err := os.ReadFile("action.yml")
	require.NoError(t, err)
	var action struct {
		Runs struct {
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &action))
	steps := action.Runs.Steps
	require.NotEmpty(t, steps)

	claim := steps[0]
	assert.Equal(t, "claim", claim.ID, "the claim must come before the download")
	assert.Equal(t, "wow-look-at-my/actions@run-once#latest", claim.Uses)
	assert.Equal(t, "slopfix-check", claim.With["name"])

	for _, id := range []string{"download", "check"} {
		idx := slices.IndexFunc(steps, func(s actionStep) bool { return s.ID == id })
		require.GreaterOrEqual(t, idx, 0, "step %s is missing", id)
		assert.Contains(t, steps[idx].If, "steps.claim.outputs.first != 'false'",
			"step %s must skip in a job that lost the claim", id)
	}
}

// checkRunScript answers the run script of the action's check step.
func checkRunScript(t *testing.T) string {
	content, err := os.ReadFile("action.yml")
	require.NoError(t, err)
	var action struct {
		Runs struct {
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &action))
	for _, step := range action.Runs.Steps {
		if step.ID == "check" {
			return step.Run
		}
	}
	t.Fatal("action.yml has no check step")
	return ""
}

// The findings of a check go to the log only. GitHub caps a step's outputs, and
// a large repository's findings in GITHUB_OUTPUT failed the step.
func TestTheActionKeepsCheckFindingsOutOfItsOutputs(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "slopfix")
	require.NoError(t, os.WriteFile(stub, []byte("echo \"finding for $*\"\n"), 0o755))
	script := strings.ReplaceAll(checkRunScript(t), "${{ steps.download.outputs.path }}", stub)

	outputs := filepath.Join(dir, "outputs")
	require.NoError(t, os.WriteFile(outputs, nil, 0o644))
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outputs)
	log, err := cmd.CombinedOutput()
	require.NoError(t, err, string(log))
	assert.Contains(t, string(log), "finding for", "the log keeps what slopfix wrote")
	written, err := os.ReadFile(outputs)
	require.NoError(t, err)
	assert.Empty(t, string(written), "a check writes no output")
}
