package workflow_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// repaired runs every workflow repair over a document.
func repaired(t *testing.T, content string) string {
	t.Helper()
	repair := workflow.Fix(content, func(string) bool { return true })
	return repair.Text
}

func TestTheGuardedJobKeyIsRenamed(t *testing.T) {
	out := repaired(t, "on: push\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n")

	assert.Contains(t, out, "  builds:")
	assert.NotContains(t, out, "all-builds")
}

func TestANeedsEntryPointingAtTheGuardedJobFollowsIt(t *testing.T) {
	out := repaired(t, "on: push\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n  ship:\n    needs: [all-builds]\n    runs-on: ubuntu-latest\n")

	assert.Contains(t, out, "needs: [builds]")
	assert.NotContains(t, out, "all-builds")
}

func TestANeedsSequenceKeepsItsOtherEntries(t *testing.T) {
	out := repaired(t, "on: push\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n  ship:\n    needs:\n      - lint\n      - all-builds\n    runs-on: ubuntu-latest\n")

	assert.Contains(t, out, "      - lint")
	assert.Contains(t, out, "      - builds")
	assert.NotContains(t, out, "all-builds")
}

// The name inside a script is text the author wrote, and a repair that reaches
// into it changes what the step runs.
func TestTheNameInsideARunScriptIsLeftAlone(t *testing.T) {
	const script = "on: push\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n          gh api repos/o/r/commits/$SHA/status --jq '.statuses[] | select(.context == \"all-builds\")'\n          echo done\n"

	out := repaired(t, script)

	assert.Contains(t, out, "  builds:")
	assert.Contains(t, out, `select(.context == "all-builds")`)
	assert.Equal(t, 1, strings.Count(out, "all-builds"))
}

// A step name is a label a person reads, not a reference the runner resolves.
func TestAStepNameCarryingTheWordsIsLeftAlone(t *testing.T) {
	out := repaired(t, "on: push\njobs:\n  gate:\n    runs-on: ubuntu-latest\n    steps:\n      - name: wait for all-builds\n        run: true\n")

	assert.Contains(t, out, "- name: wait for all-builds")
}

func TestAWorkflowWithoutTheGuardedNameIsUntouched(t *testing.T) {
	const clean = "on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n"

	repair := workflow.Fix(clean, func(string) bool { return true })

	require.False(t, repair.Changed)
	assert.Equal(t, clean, repair.Text)
}
