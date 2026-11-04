// yaml/branch-pin: a uses: ref that names a branch other than the default branch of the action's repository.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/branch-pin: a uses: ref that names a branch other than the default branch of the action's repository. A merge deletes the branch, and every job that names it then fails to resolve the action. The repair writes the default branch. The rule asks the GitHub API, and its cases run against the fixture API that Materialize serves.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDBranchPin,
		Category: RuleWorkflow,
		Detect:   detectBranchPin,
		Autofix:  autofixBranchPin,
		Cases: []RuleCase{
			branchPinCase("branch-pin", "acme/tool@feature-x", false),
			branchPinCase("branch-pin-default-and-tag", "acme/tool@master\n      - uses: acme/other@v1", true),
			branchPinCase("branch-pin-main-default", "acme/main-tool@main", true),
			branchPinRepairCase("branch-pin-main-default-feature", "acme/main-tool@feature-x", "acme/main-tool@main"),
		},
	})
}

// branchPinCase is a workflow whose uses. Steps are judged over a table that
// makes master and feature-x branches of acme/tool, and v1 a tag of
// acme/other. The default branch of acme/main-tool is main. A case marked
// kept is left as written.
func branchPinCase(name, uses string, kept bool) RuleCase {
	c := workflowCase(name, workflowHeader()+branchPinCaseYml+
		"    steps:\n      - uses: "+uses+"\n")
	c.Unchanged = kept
	c.Refs = map[string]string{
		"acme/tool":           "master",
		"acme/other":          "master",
		"acme/tool@master":    "branch",
		"acme/tool@feature-x": "branch",
		"acme/other@v1":       "tag",
		"acme/main-tool":           "main",
		"acme/main-tool@main":      "branch",
		"acme/main-tool@feature-x": "branch",
	}
	return c
}

// branchPinRepairCase is a branchPinCase whose repair must write want.
func branchPinRepairCase(name, uses, want string) RuleCase {
	c := branchPinCase(name, uses, false)
	c.Want = workflowHeader() + branchPinCaseYml + "    steps:\n      - uses: " + want + "\n"
	return c
}

// detectBranchPin answers every branch pin the walk of the case reports.
func detectBranchPin(c RuleCase) []ste.Finding {
	return treeFindings(c, workflow.IDBranchPin)
}

// autofixBranchPin writes the default branch over every branch pin of the case.
func autofixBranchPin(c RuleCase) RuleCase {
	return treeAutofix(c, workflow.IDBranchPin)
}

//go:embed testdata/branch-pin-case.yml
var branchPinCaseYml string
