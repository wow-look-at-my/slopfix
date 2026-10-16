package slopfix

import (
	"fmt"
	"os"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/branch-pin: a uses: ref that names a branch other than the default
// branch of the action's repository. A merge deletes the branch, and every job
// that names it then fails to resolve the action. The repair writes the
// default branch. The rule asks the GitHub API, so its cases answer from a
// fixed table.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDBranchPin,
		Category: RuleWorkflow,
		Detect:   detectBranchPin,
		Autofix:  autofixBranchPin,
		Cases: []RuleCase{
			workflowCase("branch-pin", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - uses: acme/tool@feature-x\n"),
			branchPinKeptCase(),
		},
	})
}

// branchPinKeptCase is a workflow whose refs the rule leaves as written: the default branch and a tag.
func branchPinKeptCase() RuleCase {
	c := workflowCase("branch-pin-default-and-tag", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
		"    steps:\n      - uses: acme/tool@main\n      - uses: acme/other@v1\n")
	c.Unchanged = true
	return c
}

// fixtureRefs answers the lookups a case needs.
type fixtureRefs struct{}

func (fixtureRefs) DefaultBranch(string) (string, error) { return "main", nil }

func (fixtureRefs) Kind(_, ref string) (workflow.RefKind, error) {
	if ref == "main" || ref == "feature-x" {
		return workflow.RefBranch, nil
	}
	return workflow.RefTag, nil
}

// detectBranchPin answers every branch pin in a workflow file of the case.
func detectBranchPin(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, path := range fixtureFiles(c) {
		text := readFixture(path)
		if !isWorkflow(path, text) {
			continue
		}
		found, err := workflow.BranchPins(text, fixtureRefs{})
		if err != nil {
			panic(fmt.Sprintf("rule case %q: %v", c.Name, err))
		}
		out = append(out, found...)
	}
	return out
}

// autofixBranchPin writes the default branch over every branch pin in a workflow file of the case.
func autofixBranchPin(c RuleCase) RuleCase {
	for _, path := range fixtureFiles(c) {
		text := readFixture(path)
		if !isWorkflow(path, text) {
			continue
		}
		fixed, err := workflow.RepairBranchPins(text, fixtureRefs{})
		if err != nil {
			panic(fmt.Sprintf("rule case %q: %v", c.Name, err))
		}
		if fixed == text {
			continue
		}
		if err := os.WriteFile(path, []byte(fixed), 0o644); err != nil {
			panic(err)
		}
	}
	return c
}
