// rule_yaml_privaterunner.go holds the rule yaml privaterunner code.

package slopfix

import _ "embed"

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
		return workflowHeader() + embeddedYml19 + steps
	}
	cases := []RuleCase{
		privateRunnerCase("private-runner", jobs("      - run: go-toolchain\n")),
		privateRunnerCase("private-runner-dind", jobs("      - run: docker build .\n")),
		privateRunnerCase("private-runner-docker-image", jobs("      - uses: docker://alpine:3.20\n")),
		containerActionCase(jobs("      - uses: " + ContainerActionFixture + "\n")),
		privateRunnerCase("private-runner-host-job", jobs("      - run: sudo modprobe zfs\n      - run: sudo systemctl restart containerd\n")),
		privateRunnerCase("private-runner-blind-fallback", workflowHeader()+"jobs:\n"+
			embeddedYml20+
			embeddedYml21+
			embeddedYml22+
			embeddedYml23),
		privateRunnerCase("private-runner-flow", workflowHeader()+"jobs:\n"+
			"  build: {runs-on: ubuntu-latest, steps: [{run: make}]}\n"+
			"  image: {runs-on: 'ubuntu-24.04', steps: [{uses: docker/setup-buildx-action@v3}]}\n"),
		privateRunnerCase("private-runner-matrix-linux", matrixJob("        os: [ubuntu-22.04, ubuntu-24.04]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-mixed", matrixJob("        os: [ubuntu-latest, macos-latest, windows-latest, ubuntu-24.04-arm]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-include", matrixJob(embeddedYml24, "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-docker", matrixJob("        os: [ubuntu-latest, macos-latest]\n", "      - run: docker build .\n")),
		privateRunnerCase("private-runner-matrix-beside-done", matrixJob("        os: [ubuntu-latest]\n", "      - run: make\n")+
			embeddedYml25+
			"    runs-on: ${{ (github.event.repository.visibility == 'public' || !contains(fromJSON('[\"ubuntu-latest\"]'), matrix.os)) && matrix.os || vars.CI_RUNNER || matrix.os }}\n"+
			"    steps:\n      - run: make\n"+
			embeddedYml26),
		privateRunnerCase("private-runner-matrix-flow", workflowHeader()+"jobs:\n"+
			"  build: {strategy: {matrix: {os: [ubuntu-latest, windows-latest]}}, runs-on: '${{ matrix.os }}', steps: [{run: make}]}\n"),
		privateRunnerCase("private-runner-macos", workflowHeader()+embeddedYml27),
		privateRunnerCase("private-runner-windows-if", workflowHeader()+"jobs:\n"+
			embeddedYml28+
			embeddedYml29),
		privateRunnerCase("private-runner-beside-gated", workflowHeader()+"jobs:\n"+
			embeddedYml30+
			embeddedYml31),
		privateRunnerCase("private-runner-matrix-foreign", matrixJob("        os: [macos-14, macos-15]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-split", matrixJob("        os:\n          - ubuntu-latest\n          - windows-latest\n        go: [stable]\n", "      - run: make\n")),
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
	return workflowHeader() + matrixJobYml + matrix + "    runs-on: ${{ matrix.os }}\n    steps:\n" + steps
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

//go:embed testdata/embedded-19.yml
var embeddedYml19 string

//go:embed testdata/embedded-20.yml
var embeddedYml20 string

//go:embed testdata/embedded-21.yml
var embeddedYml21 string

//go:embed testdata/embedded-22.yml
var embeddedYml22 string

//go:embed testdata/embedded-23.yml
var embeddedYml23 string

//go:embed testdata/embedded-24.yml
var embeddedYml24 string

//go:embed testdata/embedded-25.yml
var embeddedYml25 string

//go:embed testdata/embedded-26.yml
var embeddedYml26 string

//go:embed testdata/embedded-27.yml
var embeddedYml27 string

//go:embed testdata/embedded-28.yml
var embeddedYml28 string

//go:embed testdata/embedded-29.yml
var embeddedYml29 string

//go:embed testdata/embedded-30.yml
var embeddedYml30 string

//go:embed testdata/embedded-31.yml
var embeddedYml31 string

//go:embed testdata/matrix-job.yml
var matrixJobYml string
