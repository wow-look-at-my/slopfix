package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// PrivateRunnerFixture is the origin of the org repository the private-runner cases run in.
const PrivateRunnerFixture = "https://github.com/wow-look-at-my/private-runner-fixture"

// yaml/private-runner: an org job whose runner ignores the repository's
// visibility. The repair picks the runner from the visibility at run time.
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
			privateRunnerCase("private-runner-blind-fallback", workflowHeader()+"jobs:\n"+
				"  build:\n    runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }}\n    steps:\n      - run: make\n"+
				"  image:\n    runs-on: '${{ vars.CI_RUNNER_DIND || ''ubuntu-latest'' }}'\n    steps:\n      - run: make\n"+
				"  db:\n    runs-on: \"ubuntu-22.04\"\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n"+
				"  box:\n    runs-on: >-\n      ubuntu-latest\n    container: alpine\n    steps:\n      - run: make\n"),
			privateRunnerCase("private-runner-flow", workflowHeader()+"jobs:\n"+
				"  build: {runs-on: ubuntu-latest, steps: [{run: make}]}\n"+
				"  image: {runs-on: 'ubuntu-24.04', steps: [{uses: docker/setup-buildx-action@v3}]}\n"),
		},
	})
}

// privateRunnerCase is a workflow in the org fixture repository.
func privateRunnerCase(name, body string) RuleCase {
	c := workflowCase(name, body)
	c.Origin = PrivateRunnerFixture
	return c
}

// detectPrivateRunner answers each job whose runner ignores the visibility.
func detectPrivateRunner(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDPrivateRunner)
}

// autofixPrivateRunner writes the visibility expression into each such job.
func autofixPrivateRunner(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDPrivateRunner)
}
