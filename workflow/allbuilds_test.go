package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func TestAJobKeyedAllBuildsIsReported(t *testing.T) {
	findings := workflow.Check("on: push\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n")

	require.Len(t, findings, 1)
	assert.Equal(t, workflow.IDAllBuildsJob, findings[0].ID)
	assert.Equal(t, 3, findings[0].Line)
	assert.Equal(t, "its job key", findings[0].Detail)
	assert.Contains(t, findings[0].Fix, "Rename the job")
}

func TestAJobNamedAllBuildsIsReported(t *testing.T) {
	findings := workflow.Check("on: push\njobs:\n  gate:\n    name: all-builds\n    runs-on: ubuntu-latest\n")

	require.Len(t, findings, 1)
	assert.Equal(t, 4, findings[0].Line)
	assert.Equal(t, "its name", findings[0].Detail)
}

// GitHub decorates a name with a matrix suffix and with a reusable workflow's
// path parts. The decoration is not a different name.
func TestADecoratedNameStillShadowsTheGate(t *testing.T) {
	for _, name := range []string{"all-builds (ubuntu-latest)", "ci / all-builds", "all-builds / deploy"} {
		findings := workflow.Check("on: push\njobs:\n  gate:\n    name: '" + name + "'\n")
		assert.Len(t, findings, 1, name)
	}
}

func TestANameThatMerelyContainsTheWordIsAllowed(t *testing.T) {
	for _, name := range []string{"All-Builds", "all-builds2", "build-all", "all builds"} {
		assert.Empty(t, workflow.Check("on: push\njobs:\n  gate:\n    name: '"+name+"'\n"), name)
	}
}

// An expression resolves on the runner, so no file can judge it.
func TestAnExpressionNameIsLeftAlone(t *testing.T) {
	assert.Empty(t, workflow.Check("on: push\njobs:\n  gate:\n    name: ${{ matrix.target }}\n"))
}

func TestUnparseableYAMLReportsNoJob(t *testing.T) {
	findings := workflow.Check("jobs:\n  - [unbalanced\n")

	for _, finding := range findings {
		assert.NotEqual(t, workflow.IDAllBuildsJob, finding.ID)
	}
}

func TestAFileWithNoJobsReportsNothing(t *testing.T) {
	assert.Empty(t, workflow.Check("runs:\n  using: composite\n"))
}
