package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/workflow"
)

const permissionWorkflow = `on:
  push:
    branches: ['**']
permissions:
  contents: read
  id-token: write
jobs:
  own:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps: []
  inherits:
    runs-on: ubuntu-latest
    steps: []
  all:
    runs-on: ubuntu-latest
    permissions: write-all
    steps: []
`

func TestTheJobBlockWinsAndOmitsWhatItDoesNotName(t *testing.T) {
	g, err := workflow.Permission(permissionWorkflow, "own", "contents")
	require.NoError(t, err)
	assert.Equal(t, workflow.Grant{Level: "write", Source: "job"}, g)

	// A job block replaces the workflow block, so id-token falls to none.
	g, err = workflow.Permission(permissionWorkflow, "own", "id-token")
	require.NoError(t, err)
	assert.Equal(t, workflow.Grant{Level: "none", Source: "job"}, g)
	assert.False(t, g.Covers("read"))
}

func TestAJobWithNoBlockReadsTheWorkflowBlock(t *testing.T) {
	g, err := workflow.Permission(permissionWorkflow, "inherits", "id-token")
	require.NoError(t, err)
	assert.Equal(t, workflow.Grant{Level: "write", Source: "workflow"}, g)
	assert.True(t, g.Covers("read"))
}

func TestWriteAllGrantsEveryPermission(t *testing.T) {
	g, err := workflow.Permission(permissionWorkflow, "all", "packages")
	require.NoError(t, err)
	assert.Equal(t, "write", g.Level)
}

func TestNoBlockAnywhereIsTheRepositoryDefault(t *testing.T) {
	g, err := workflow.Permission("jobs:\n  a:\n    steps: []\n", "a", "contents")
	require.NoError(t, err)
	assert.Equal(t, workflow.Grant{Level: "none", Source: "default"}, g)
}

func TestABadWorkflowIsAnError(t *testing.T) {
	_, err := workflow.Permission(permissionWorkflow, "missing", "contents")
	assert.ErrorContains(t, err, `names no job "missing"`)

	_, err = workflow.Permission("jobs:\n  a:\n    permissions:\n      contents: admin\n", "a", "contents")
	assert.ErrorContains(t, err, "not read, write or none")

	_, err = workflow.Permission("jobs:\n  a:\n    permissions: some\n", "a", "contents")
	assert.ErrorContains(t, err, "neither read-all nor write-all")
}
