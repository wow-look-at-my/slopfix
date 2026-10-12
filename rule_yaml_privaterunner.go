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
	cases := []RuleCase{
		privateRunnerCase("private-runner", jobs("      - run: go-toolchain\n")),
		privateRunnerCase("private-runner-dind", jobs("      - run: docker build .\n")),
		privateRunnerCase("private-runner-docker-image", jobs("      - uses: docker://alpine:3.20\n")),
		containerActionCase(jobs("      - uses: " + ContainerActionFixture + "\n")),
		privateRunnerCase("private-runner-host-job", jobs("      - uses: vmactions/freebsd-vm@v1\n      - run: sudo modprobe zfs\n")),
		privateRunnerCase("private-runner-blind-fallback", workflowHeader()+"jobs:\n"+
			"  build:\n    runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }}\n    steps:\n      - run: make\n"+
			"  image:\n    runs-on: '${{ vars.CI_RUNNER_DIND || ''ubuntu-latest'' }}'\n    steps:\n      - run: make\n"+
			"  db:\n    runs-on: \"ubuntu-22.04\"\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n"+
			"  box:\n    runs-on: >-\n      ubuntu-latest\n    container: alpine\n    steps:\n      - run: make\n"),
		privateRunnerCase("private-runner-flow", workflowHeader()+"jobs:\n"+
			"  build: {runs-on: ubuntu-latest, steps: [{run: make}]}\n"+
			"  image: {runs-on: 'ubuntu-24.04', steps: [{uses: docker/setup-buildx-action@v3}]}\n"),
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDPrivateRunner,
		Category: RuleWorkflow,
		Detect:   detectPrivateRunner,
		Autofix:  autofixPrivateRunner,
		Cases:    cases,
	})
}

// ContainerActionFixture is a third-party action that the fixture records as a container action.
const ContainerActionFixture = "someone/notify-irc@v2"

// containerActionCase is a workflow whose step uses ContainerActionFixture.
func containerActionCase(body string) RuleCase {
	c := privateRunnerCase("private-runner-container-action", body)
	c.Actions = map[string]string{ContainerActionFixture: "docker"}
	return c
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
