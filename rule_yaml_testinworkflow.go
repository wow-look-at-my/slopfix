package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/test-in-workflow: a test written into a run: script. A line of the
// script is shell, so the repair cuts none. It moves the whole script into a
// file under .github/scripts, and the step runs that file. Each ${{ }}
// expression in the script becomes an argument of the command.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDTestInYAML,
		Category: RuleWorkflow,
		Detect:   detectTestInYAML,
		Autofix:  autofixTestInYAML,
		Cases: []RuleCase{
			workflowCase("test-in-workflow", testInYAMLCase()),
			workflowCase("test-in-workflow-expression", testInYAMLExpressionCase()),
		},
	})
}

// detectTestInYAML answers every test written into a run: script.
func detectTestInYAML(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDTestInYAML)
}

// autofixTestInYAML moves each run: script that holds a test into a file, as
// `slopfix fix` does on the tree.
func autofixTestInYAML(c RuleCase) RuleCase {
	return treeAutofix(c, workflow.IDTestInYAML)
}

// testInYAMLCase is a workflow whose step asserts on a value beside its own
// command.
func testInYAMLCase() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n" +
		"    steps:\n      - run: |\n          echo hi\n          assert_ok() { exit 1; }\n"
}

// testInYAMLExpressionCase is a workflow whose test reads an expression inside
// a guard, which a cut of the line that exits would leave empty.
func testInYAMLExpressionCase() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n" +
		"    steps:\n      - run: |\n" +
		"          if ! grep -q \"${{ github.sha }}\" out.txt; then\n" +
		"            echo \"::error::no build of ${{ github.sha }}\"; exit 1\n" +
		"          fi\n"
}
