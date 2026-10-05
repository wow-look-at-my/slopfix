package slopfix_test

import (
	"os"
	"slices"
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

type actionStep struct {
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
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
	assert.Equal(t, "inputs.permission == ''", claim.If, "the permission answer is per job and must never be claimed")

	for _, id := range []string{"download", "check"} {
		idx := slices.IndexFunc(steps, func(s actionStep) bool { return s.ID == id })
		require.GreaterOrEqual(t, idx, 0, "step %s is missing", id)
		assert.Contains(t, steps[idx].If, "steps.claim.outputs.first != 'false'",
			"step %s must skip in a job that lost the claim", id)
	}
}
