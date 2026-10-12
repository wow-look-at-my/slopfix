package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// PrivateRunnerFixture is the origin of the private org repository the private-runner cases run in.
const PrivateRunnerFixture = "https://github.com/wow-look-at-my/private-runner-fixture"

// yaml/private-runner: a job of a private org repository on a GitHub-hosted
// Linux runner. The repair sends it to the org's own runners.
func init() {
	jobs := func(steps string) string {
		return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n" + steps
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDPrivateRunner,
		Category: RuleWorkflow,
		Detect:   detectPrivateRunner,
		Autofix:  autofixPrivateRunner,
		Cases: []RuleCase{
			privateRunnerCase("private-runner", jobs("      - run: go-toolchain\n")),
			privateRunnerCase("private-runner-dind", jobs("      - run: docker build .\n")),
		},
	})
}

// privateRunnerCase is a workflow in the private fixture repository.
func privateRunnerCase(name, body string) RuleCase {
	c := workflowCase(name, body)
	c.Origin, c.Private = PrivateRunnerFixture, true
	return c
}

// detectPrivateRunner answers each job this case runs on a GitHub-hosted runner.
func detectPrivateRunner(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDPrivateRunner)
}

// autofixPrivateRunner sends each such job to the org's own runners.
func autofixPrivateRunner(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDPrivateRunner)
}
