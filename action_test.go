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
