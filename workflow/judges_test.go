package workflow_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/workflow"
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
	assert.True(t, workflow.Sniff(testSniffFindsAWorkflowThePathDoesNotNameYml))
	assert.True(t, workflow.Sniff(testSniffFindsAWorkflowThePathDoesNotNameYml2))
}

func TestSniffLeavesAnOrdinaryDocumentAlone(t *testing.T) {
	assert.False(t, workflow.Sniff(testSniffLeavesAnOrdinaryDocumentAloneYml))
	assert.False(t, workflow.Sniff("# jobs: the ones we run\ntext: here\n"))
}

//go:embed testdata/test-sniff-finds-a-workflow-the-path-does-not-name.yml
var testSniffFindsAWorkflowThePathDoesNotNameYml string

//go:embed testdata/test-sniff-finds-a-workflow-the-path-does-not-name-2.yml
var testSniffFindsAWorkflowThePathDoesNotNameYml2 string

//go:embed testdata/test-sniff-leaves-an-ordinary-document-alone.yml
var testSniffLeavesAnOrdinaryDocumentAloneYml string
