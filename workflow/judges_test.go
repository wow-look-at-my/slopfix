package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfmt/workflow"
)

func TestJudgesReadsAWorkflowAndAnActionManifest(t *testing.T) {
	for _, name := range []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yaml",
		"action.yml",
		"orphan-release/action.yaml",
		`.github\workflows\ci.yml`,
	} {
		assert.True(t, workflow.Judges(name), name)
	}
}

func TestJudgesLeavesEveryOtherFileAlone(t *testing.T) {
	for _, name := range []string{
		"README.md",
		"docs/ci.md",
		"compose.yaml",
		"config/settings.yml",
		".github/dependabot.yml",
	} {
		assert.False(t, workflow.Judges(name), name)
	}
}

// A path says which file this is most of the time. It does not when the caller
// names a file from inside the workflows directory.
func TestSniffFindsAWorkflowThePathDoesNotName(t *testing.T) {
	assert.True(t, workflow.Sniff("on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n"))
	assert.True(t, workflow.Sniff("name: Thing\nruns:\n  using: composite\n"))
}

func TestSniffLeavesAnOrdinaryDocumentAlone(t *testing.T) {
	assert.False(t, workflow.Sniff("services:\n  web:\n    image: nginx\n"))
	assert.False(t, workflow.Sniff("# jobs: the ones we run\ntext: here\n"))
}
