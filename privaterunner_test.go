package slopfix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// runnerRepo writes a workflow into a repository with origin, and answers its path.
func runnerRepo(t *testing.T, origin, body string) string {
	t.Helper()
	c, err := Materialize(t.TempDir(), RuleCase{Name: "runner", Path: ".github/workflows/ci.yml", Text: body, Origin: origin})
	require.NoError(t, err)
	return filepath.Join(c.Root, ".github", "workflows", "ci.yml")
}

// visible is the runs-on expression the repair writes for label and the org runner variable.
func visible(label, variable string) string {
	return "${{ github.event.repository.visibility == 'public' && '" + label + "' || vars." + variable + " || '" + label + "' }}"
}

func TestPrivateRunnerRewritesEachHostedLinuxJob(t *testing.T) {
	body := workflowHeader() + "jobs:\n" +
		"  build:\n    runs-on: ubuntu-24.04 # pinned\n    steps:\n      - run: make\n" +
		"  image:\n    runs-on: 'ubuntu-latest'\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n" +
		"  old:\n    runs-on: ${{ vars.CI_RUNNER || 'ubuntu-latest' }} # org runner\n    steps:\n      - run: docker build .\n" +
		"  kept:\n    runs-on: \"${{ vars.CI_RUNNER_DIND||'ubuntu-22.04' }}\"\n    steps:\n      - run: make\n" +
		"  tagged:\n    runs-on: &runner !!str ubuntu-latest\n    steps:\n      - run: make\n" +
		"  plain:\n    runs-on: ubuntu-latest\n\n    # the steps\n    steps:\n      - run: make\n" +
		"  flow: {runs-on: \"ubuntu-\\\n      latest\", steps: [{run: make}]}\n" +
		"  mac:\n    runs-on: macos-latest\n    steps:\n      - run: make\n" +
		"  arm:\n    runs-on: ubuntu-24.04-arm\n    steps:\n      - run: make\n" +
		"  matrix:\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  done:\n    runs-on: " + visible("ubuntu-latest", "CI_RUNNER") + "\n    steps:\n      - run: make\n" +
		"  push:\n    runs-on: ubuntu-20.04\n    steps:\n      - uses: wow-look-at-my/actions@ghcr#latest\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 8)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-20.04", "CI_RUNNER_DIND")+"\n", "the ghcr action builds with docker")
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-24.04", "CI_RUNNER")+" # pinned\n")
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", "CI_RUNNER_DIND")+"\n    services:")
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", "CI_RUNNER_DIND")+" # org runner\n", "a docker step picks the docker runners")
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-22.04", "CI_RUNNER_DIND")+"\n", "the docker runners stay when the author chose them")
	assert.Contains(t, got.Text, "runs-on: &runner !!str "+visible("ubuntu-latest", "CI_RUNNER")+"\n", "the tag and anchor stay")
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", "CI_RUNNER")+"\n\n    # the steps\n")
	assert.Contains(t, got.Text, "  flow: {runs-on: \""+visible("ubuntu-latest", "CI_RUNNER")+"\", steps: [{run: make}]}\n")
	assert.Contains(t, got.Text, "runs-on: macos-latest\n", "the org runners are Linux")
	assert.Contains(t, got.Text, "runs-on: ubuntu-24.04-arm\n", "the org runners are x64")
	assert.Contains(t, got.Text, "runs-on: ${{ matrix.os }}\n", "another expression is not read")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

func TestPrivateRunnerReadsNoVisibility(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n"
	path := runnerRepo(t, "https://github.com/wow-look-at-my/public-runner-fixture", body)
	event := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(event, []byte(`{"repository":{"full_name":"wow-look-at-my/public-runner-fixture","private":false,"visibility":"public"}}`), 0o644))
	t.Setenv("GITHUB_EVENT_PATH", event)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 1, "a public repository gets the expression too")
}

func TestPrivateRunnerSkipsRepositoriesItDoesNotGovern(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n"
	for name, origin := range map[string]string{
		"no origin":        "",
		"another owner":    "https://github.com/someone/private-runner-fixture",
		"exempt":           "https://github.com/wow-look-at-my/webhook-runner",
		"another git host": "https://example.com/wow-look-at-my/private-runner-fixture",
	} {
		path := runnerRepo(t, origin, body)
		assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner), name)
	}
}

func TestPrivateRunnerSendsAContainerStepToTheDockerRunner(t *testing.T) {
	steps := map[string]struct {
		uses, using, want string
	}{
		"docker image":     {"docker://alpine:3.20", "", runnerDind},
		"container action": {"someone/container-action@v1", "docker", runnerDind},
		"node action":      {"someone/node-action/sub@v3", "node20", runnerSlim},
	}
	for name, step := range steps {
		body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: " + step.uses + "\n"
		c := RuleCase{Name: name, Path: ".github/workflows/ci.yml", Text: body, Origin: PrivateRunnerFixture}
		if step.using != "" {
			c.Actions = map[string]string{step.uses: step.using}
		}
		c, err := Materialize(t.TempDir(), c)
		require.NoError(t, err)
		path := filepath.Join(c.Root, ".github", "workflows", "ci.yml")
		got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
		require.NoError(t, err)
		assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", step.want)+"\n", name)
	}
}

func TestPrivateRunnerChoosesTheDockerRunnerForAnUnreadAction(t *testing.T) {
	t.Setenv("GITHUB_API_URL", "http://127.0.0.1:1")
	t.Setenv("GITHUB_TOKEN", "fixture")
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: someone/unread-action@v1\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	found := only(CheckContent(path, body), workflow.IDPrivateRunner)
	require.Len(t, found, 1)
	assert.Contains(t, found[0].Detail, "the action metadata could not be read")
	assert.Contains(t, found[0].Detail, "someone/unread-action@v1")
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", runnerDind)+"\n")
}

func TestPrivateRunnerKeepsAJobThatNeedsTheHost(t *testing.T) {
	for name, steps := range HostJobSteps {
		body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n" + hostJob(steps)
		path := runnerRepo(t, PrivateRunnerFixture, body)
		found := only(CheckContent(path, body), workflow.IDPrivateRunner)
		require.Len(t, found, 1, name)
		assert.Contains(t, found[0].Detail, "runs-on: ubuntu-latest", name)
		got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
		require.NoError(t, err)
		assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-latest", runnerSlim)+"\n", name)
		assert.Contains(t, got.Text, "  host:\n    runs-on: ubuntu-24.04", name)
		assert.NotContains(t, got.Text, "|| 'ubuntu-24.04'", name)
	}
}

func TestPrivateRunnerLeavesAHostJobOnTheOrgRunnersAlone(t *testing.T) {
	body := workflowHeader() + "jobs:\n  zloop:\n    runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }}\n    steps:\n      - run: sudo modprobe zfs\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner), "a host job is neither reported nor written back")
}

func TestPrivateRunnerMovesAJobThatOnlyMentionsTheHost(t *testing.T) {
	for name, steps := range map[string]string{
		"an echo":            "      - run: echo modprobe swapoff\n",
		"a user package":     "      - run: sudo apt-get install -y libsystemd-dev\n",
		"a header elsewhere": "      - run: ls linux-headers-6.8\n",
		"a quoted word":      "      - run: grep -r 'systemctl' docs\n",
	} {
		body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n" + steps
		path := runnerRepo(t, PrivateRunnerFixture, body)
		assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 1, name)
	}
}

func TestPrivateRunnerNamesAnOptOutWithNoReason(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest # " + workflow.HostedMarker + "\n    steps:\n      - run: make\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	found := only(CheckContent(path, body), workflow.IDPrivateRunner)
	require.Len(t, found, 1)
	assert.Contains(t, found[0].Detail, "gives no reason")
}

// The org runner variables, as the repair writes them.
const (
	runnerSlim = "CI_RUNNER"
	runnerDind = "CI_RUNNER_DIND"
)
