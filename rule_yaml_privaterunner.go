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
		return workflowHeader() + embeddedYml33 + steps
	}
	cases := []RuleCase{
		privateRunnerCase("private-runner", jobs("      - run: go-toolchain\n")),
		privateRunnerCase("private-runner-dind", jobs("      - run: docker build .\n")),
		privateRunnerCase("private-runner-docker-image", jobs("      - uses: docker://alpine:3.20\n")),
		containerActionCase(jobs("      - uses: " + ContainerActionFixture + "\n")),
		privateRunnerCase("private-runner-host-job", jobs("      - run: sudo modprobe zfs\n      - run: sudo systemctl restart containerd\n")),
		privateRunnerCase("private-runner-blind-fallback", workflowHeader()+"jobs:\n"+
			embeddedYml34+
			embeddedYml35+
			embeddedYml36+
			embeddedYml37),
		privateRunnerCase("private-runner-flow", workflowHeader()+"jobs:\n"+
			"  build: {runs-on: ubuntu-latest, steps: [{run: make}]}\n"+
			"  image: {runs-on: 'ubuntu-24.04', steps: [{uses: docker/setup-buildx-action@v3}]}\n"),
		privateRunnerCase("private-runner-matrix-linux", matrixJob("        os: [ubuntu-22.04, ubuntu-24.04]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-mixed", matrixJob("        os: [ubuntu-latest, macos-latest, windows-latest, ubuntu-24.04-arm]\n", "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-include", matrixJob(embeddedYml38, "      - run: make\n")),
		privateRunnerCase("private-runner-matrix-docker", matrixJob("        os: [ubuntu-latest, macos-latest]\n", "      - run: docker build .\n")),
		privateRunnerCase("private-runner-matrix-beside-done", matrixJob("        os: [ubuntu-latest]\n", "      - run: make\n")+
			embeddedYml39+
			"    runs-on: ${{ (github.event.repository.visibility == 'public' || !contains(fromJSON('[\"ubuntu-latest\"]'), matrix.os)) && matrix.os || vars.CI_RUNNER || matrix.os }}\n"+
			"    steps:\n      - run: make\n"+
			embeddedYml40),
		privateRunnerCase("private-runner-matrix-flow", workflowHeader()+"jobs:\n"+
			"  build: {strategy: {matrix: {os: [ubuntu-latest, windows-latest]}}, runs-on: '${{ matrix.os }}', steps: [{run: make}]}\n"),
		privateRunnerCase("private-runner-macos", workflowHeader()+embeddedYml41),
		privateRunnerCase("private-runner-windows-if", workflowHeader()+"jobs:\n"+
			embeddedYml42+
			embeddedYml43),
		privateRunnerCase("private-runner-beside-gated", workflowHeader()+"jobs:\n"+
			embeddedYml44+
			embeddedYml45),
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

//go:embed testdata/embedded-9.yml
var embeddedYml33 string

//go:embed testdata/embedded-29.yml
var embeddedYml34 string

//go:embed testdata/embedded-30.yml
var embeddedYml35 string

//go:embed testdata/embedded-31.yml
var embeddedYml36 string

//go:embed testdata/embedded-32.yml
var embeddedYml37 string

//go:embed testdata/embedded-33.yml
var embeddedYml38 string

//go:embed testdata/embedded-34.yml
var embeddedYml39 string

//go:embed testdata/embedded-35.yml
var embeddedYml40 string

//go:embed testdata/embedded-36.yml
var embeddedYml41 string

//go:embed testdata/embedded-37.yml
var embeddedYml42 string

//go:embed testdata/embedded-38.yml
var embeddedYml43 string

//go:embed testdata/embedded-39.yml
var embeddedYml44 string

//go:embed testdata/embedded-40.yml
var embeddedYml45 string

//go:embed testdata/matrix-job.yml
var matrixJobYml string
