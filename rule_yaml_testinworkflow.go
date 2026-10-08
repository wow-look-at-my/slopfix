package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/test-in-workflow: a test written into a run: script. The step keeps its
// own commands. The repair takes out the lines that carry the test, because a
// test belongs in the repository's suite where a test runner reaches it.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDTestInYAML,
		Category: RuleWorkflow,
		Detect:   detectTestInYAML,
		Autofix:  autofixTestInYAML,
		Cases:    []RuleCase{workflowCase("test-in-workflow", testInYAMLCase())},
	})
}

// detectTestInYAML answers every test written into a run: script.
func detectTestInYAML(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDTestInYAML)
}

// autofixTestInYAML takes the test lines out of the run: script.
func autofixTestInYAML(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDTestInYAML)
}

// testInYAMLCase is a workflow whose step asserts on a value beside its own
// command.
func testInYAMLCase() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n" +
		"    steps:\n      - run: |\n          echo hi\n          assert_ok() { exit 1; }\n"
}
