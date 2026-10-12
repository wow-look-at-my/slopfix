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
		privateRunnerCase("private-runner-host-job", jobs("      - run: sudo modprobe zfs\n      - run: sudo systemctl restart containerd\n")),
		privateRunnerCase("private-runner-blind-fallback", workflowHeader()+"jobs:\n"+
			"  build:\n    runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }}\n    steps:\n      - run: make\n"+
			"  image:\n    runs-on: '${{ vars.CI_RUNNER_DIND || ''ubuntu-latest'' }}'\n    steps:\n      - run: make\n"+
			"  db:\n    runs-on: \"ubuntu-22.04\"\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n"+
			"  box:\n    runs-on: >-\n      ubuntu-latest\n    container: alpine\n    steps:\n      - run: make\n"),
		privateRunnerCase("private-runner-flow", workflowHeader()+"jobs:\n"+
			"  build: {runs-on: ubuntu-latest, steps: [{run: make}]}\n"+
			"  image: {runs-on: 'ubuntu-24.04', steps: [{uses: docker/setup-buildx-action@v3}]}\n"),
		privateRunnerCase("private-runner-matrix-linux", matrixJob("        os: [ubuntu-22.04, ubuntu-24.04]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-mixed", matrixJob("        os: [ubuntu-latest, macos-latest, windows-latest, ubuntu-24.04-arm]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-include", matrixJob("        include:\n          - {os: ubuntu-22.04, arch: amd64}\n          - {os: ubuntu-24.04-arm, arch: arm64}\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-docker", matrixJob("        os: [ubuntu-latest, macos-latest]\n", "      - run: docker build .\n")),
		privateRunnerCase("private-runner-matrix-beside-done", matrixJob("        os: [ubuntu-latest]\n", "      - run: make\n")+
			"  done:\n    strategy:\n      matrix:\n        os: [ubuntu-latest, macos-latest]\n"+
			"    runs-on: ${{ (github.event.repository.visibility == 'public' || !contains(fromJSON('[\"ubuntu-latest\"]'), matrix.os)) && matrix.os || vars.CI_RUNNER || matrix.os }}\n"+
			"    steps:\n      - run: make\n"+
			"  dynamic:\n    strategy:\n      matrix: ${{ fromJSON(needs.plan.outputs.matrix) }}\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n"),
		privateRunnerCase("private-runner-matrix-flow", workflowHeader()+"jobs:\n"+
			"  build: {strategy: {matrix: {os: [ubuntu-latest, windows-latest]}}, runs-on: '${{ matrix.os }}', steps: [{run: make}]}\n"),
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDPrivateRunner,
		Category: RuleWorkflow,
		Detect:   detectPrivateRunner,
		Autofix:  autofixPrivateRunner,
		Cases:    cases,
	})
}

// matrixJob is a workflow whose build job takes its runner from the matrix value os.
func matrixJob(matrix, steps string) string {
	return workflowHeader() + "jobs:\n  build:\n    strategy:\n      matrix:\n" + matrix + "    runs-on: ${{ matrix.os }}\n    steps:\n" + steps
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
