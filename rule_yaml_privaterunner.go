package slopfix

import (
	"maps"
	"slices"
	"strings"

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
	cases := []RuleCase{
		privateRunnerCase("private-runner", jobs("      - run: go-toolchain\n")),
		privateRunnerCase("private-runner-dind", jobs("      - run: docker build .\n")),
		privateRunnerCase("private-runner-docker-image", jobs("      - uses: docker://alpine:3.20\n")),
		containerActionCase(jobs("      - uses: " + ContainerActionFixture + "\n")),
		privateRunnerCase("private-runner-unexplained-opt-out", strings.Replace(jobs("      - run: make\n"),
			"ubuntu-latest", "ubuntu-latest # "+workflow.HostedMarker, 1)),
	}
	for _, name := range slices.Sorted(maps.Keys(HostJobSteps)) {
		cases = append(cases, privateRunnerCase("private-runner-host-"+name, jobs("      - run: make\n")+hostJob(HostJobSteps[name])))
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDPrivateRunner,
		Category: RuleWorkflow,
		Detect:   detectPrivateRunner,
		Autofix:  autofixPrivateRunner,
		Cases:    cases,
	})
}

// HostJobSteps are the steps of a job that needs the host, by the sign it shows.
// Each case pairs such a job with an ordinary one, which the repair still moves.
var HostJobSteps = map[string]string{
	"vm-action":      "      - uses: vmactions/freebsd-vm@v1\n",
	"k3s-action":     "      - uses: crazy-max/.github/.github/actions/install-k3s@v1.8.0\n",
	"kernel-module":  "      - run: sudo modprobe zfs\n",
	"kernel-headers": "      - run: sudo apt-get install -y linux-headers-$(uname -r)\n",
	"swap":           "      - run: |\n          sudo swapoff -a\n          make e2e\n",
	"systemd":        "      - run: sudo systemctl restart containerd\n",
	"kvm":            "      - run: qemu-system-x86_64 -enable-kvm -m 2G disk.img\n",
	"kvm-device":     "      - run: sudo chmod 666 /dev/kvm\n",
	"opt-out":        "      - run: .github/scripts/boot.sh\n",
}

// hostJob is a job named host with steps. The opt-out case keeps its label by a comment instead.
func hostJob(steps string) string {
	on := "ubuntu-24.04"
	if strings.Contains(steps, "boot.sh") {
		on += " # " + workflow.HostedMarker + " the script boots a KVM guest"
	}
	return "  host:\n    runs-on: " + on + "\n    steps:\n" + steps
}

// ContainerActionFixture is a third-party action that the fixture records as a container action.
const ContainerActionFixture = "someone/notify-irc@v2"

// containerActionCase is a workflow whose step uses ContainerActionFixture.
func containerActionCase(body string) RuleCase {
	c := privateRunnerCase("private-runner-container-action", body)
	c.Actions = map[string]string{ContainerActionFixture: "docker"}
	return c
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
