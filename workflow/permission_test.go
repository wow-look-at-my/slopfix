package workflow_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/workflow"
)

//go:embed testdata/permission-workflow.yml
var permissionWorkflow string

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
	g, err := workflow.Permission(testNoBlockAnywhereIsTheRepositoryDefaultYml, "a", "contents")
	require.NoError(t, err)
	assert.Equal(t, workflow.Grant{Level: "none", Source: "default"}, g)
}

func TestABadWorkflowIsAnError(t *testing.T) {
	_, err := workflow.Permission(permissionWorkflow, "missing", "contents")
	assert.ErrorContains(t, err, `names no job "missing"`)

	_, err = workflow.Permission(testABadWorkflowIsAnErrorYml, "a", "contents")
	assert.ErrorContains(t, err, "not read, write or none")

	_, err = workflow.Permission(testABadWorkflowIsAnErrorYml2, "a", "contents")
	assert.ErrorContains(t, err, "neither read-all nor write-all")
}

//go:embed testdata/test-no-block-anywhere-is-the-repository-default.yml
var testNoBlockAnywhereIsTheRepositoryDefaultYml string

//go:embed testdata/test-a-bad-workflow-is-an-error.yml
var testABadWorkflowIsAnErrorYml string

//go:embed testdata/test-a-bad-workflow-is-an-error-2.yml
var testABadWorkflowIsAnErrorYml2 string
