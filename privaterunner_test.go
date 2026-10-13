package slopfix

import (
	"os"
	"path/filepath"
	"strings"
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
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 10)
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
	assert.Contains(t, got.Text, "  mac:\n    if: "+publicOnly+"\n    runs-on: macos-latest\n", "the org runners are Linux")
	assert.Contains(t, got.Text, "  arm:\n    if: "+publicOnly+"\n    runs-on: ubuntu-24.04-arm\n", "the org runners are x64")
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

func TestPrivateRunnerMovesAJobThatNeedsTheHost(t *testing.T) {
	for name, steps := range map[string]string{
		"kernel module": "      - run: sudo modprobe zfs\n",
		"swap":          "      - run: sudo swapoff -a\n",
		"systemd":       "      - run: sudo systemctl restart containerd\n",
		"kvm":           "      - run: qemu-system-x86_64 -enable-kvm -m 2G disk.img\n",
	} {
		body := workflowHeader() + "jobs:\n  host:\n    runs-on: ubuntu-24.04 # slopfix: hosted the job boots a guest\n    steps:\n" + steps
		path := runnerRepo(t, PrivateRunnerFixture, body)
		require.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 1, name)
		got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
		require.NoError(t, err)
		assert.Contains(t, got.Text, "runs-on: "+visible("ubuntu-24.04", runnerSlim), name)
	}
}

// The org runner variables, as the repair writes them.
const (
	runnerSlim = "CI_RUNNER"
	runnerDind = "CI_RUNNER_DIND"
)

// matrixVisible is the runs-on expression the repair writes for a matrix key and its hosted Linux values.
func matrixVisible(key, list, variable string) string {
	ref := "matrix." + key
	return "${{ (github.event.repository.visibility == 'public' || !contains(fromJSON('" + list + "'), " + ref + ")) && " + ref + " || vars." + variable + " || " + ref + " }}"
}

func TestPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrix(t *testing.T) {
	done := matrixVisible("os", `["ubuntu-latest"]`, runnerSlim)
	body := workflowHeader() + "jobs:\n" +
		"  linux:\n    strategy:\n      matrix:\n        include:\n          - {os: ubuntu-22.04, arch: amd64}\n          - {os: ubuntu-24.04, arch: amd64}\n          - {os: ubuntu-22.04-arm, arch: arm64}\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  mixed:\n    strategy:\n      matrix:\n        os: [ubuntu-latest, macos-latest, windows-latest]\n        include:\n          - os: ubuntu-24.04\n    runs-on: ${{matrix.os}}\n    steps:\n      - run: docker build .\n" +
		"  apple:\n    strategy:\n      matrix:\n        os: [macos-14, macos-15]\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  dynamic:\n    strategy:\n      matrix: ${{ fromJSON(needs.plan.outputs.matrix) }}\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  partial:\n    strategy:\n      matrix:\n        os: ${{ fromJSON(needs.plan.outputs.os) }}\n        include:\n          - os: ubuntu-latest\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  done:\n    strategy:\n      matrix:\n        os: [ubuntu-latest, macos-latest]\n    runs-on: " + done + "\n    steps:\n      - run: make\n" +
		"  flow: {strategy: {matrix: {os: [ubuntu-latest, windows-latest]}}, runs-on: '${{ matrix.os }}', steps: [{run: make}]}\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 6)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: "+matrixVisible("os", `["ubuntu-22.04","ubuntu-24.04"]`, runnerSlim)+"\n", "the arm values stay out of the list")
	assert.Contains(t, got.Text, "          - {os: ubuntu-22.04-arm, arch: arm64}\n", "an include entry with a foreign value is not read")
	assert.Contains(t, got.Text, "runs-on: "+matrixVisible("os", `["ubuntu-latest","ubuntu-24.04"]`, runnerDind)+"\n", "a docker job picks the docker runners")
	assert.Contains(t, got.Text, "        os: "+split(`["ubuntu-latest","macos-latest","windows-latest"]`, `["ubuntu-latest"]`)+"\n        include:\n", "a private repository drops the foreign values")
	assert.Contains(t, got.Text, "os: [macos-14, macos-15]\n    if: "+publicOnly+"\n    runs-on: ${{ matrix.os }}\n", "a matrix of foreign values keeps the job to a public repository")
	assert.Contains(t, got.Text, "        os: "+split(`["ubuntu-latest","macos-latest"]`, `["ubuntu-latest"]`)+"\n    runs-on: "+done+"\n", "a routed matrix drops its foreign values")
	assert.Contains(t, got.Text, "matrix: ${{ fromJSON(needs.plan.outputs.matrix) }}\n    runs-on: ${{ matrix.os }}\n", "a dynamic matrix is not read")
	assert.Contains(t, got.Text, "          - os: ubuntu-latest\n    runs-on: ${{ matrix.os }}\n", "a key with a dynamic list is not read")
	assert.Contains(t, got.Text, `runs-on: "`+strings.ReplaceAll(matrixVisible("os", `["ubuntu-latest"]`, runnerSlim), `"`, `\"`)+`", steps:`)
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

// publicOnly is the condition that keeps a job or a matrix value to a public repository.
const publicOnly = "github.event.repository.visibility == 'public'"

// split is the matrix list expression the repair writes for the whole list and the list a private repository runs.
func split(whole, private string) string {
	return "${{ " + publicOnly + " && fromJSON('" + whole + "') || fromJSON('" + private + "') }}"
}

func TestPrivateRunnerKeepsAForeignJobToAPublicRepository(t *testing.T) {
	body := workflowHeader() + "jobs:\n" +
		"  mac:\n    runs-on: macos-latest\n    steps:\n      - run: make\n" +
		"  win:\n    if: github.ref == 'refs/heads/master'\n    runs-on: windows-latest\n    steps:\n      - run: make\n" +
		"  wrapped:\n    if: ${{ !cancelled() }} # keep going\n    runs-on: ubuntu-24.04-arm\n    steps:\n      - run: make\n" +
		"  quoted:\n    if: \"needs.plan.outputs.go == 'yes'\"\n    runs-on: windows-11-arm\n    steps:\n      - run: make\n" +
		"  gated:\n    if: ${{ github.event.repository.visibility == 'public' && always() }}\n    runs-on: macos-14\n    steps:\n      - run: make\n" +
		"  self:\n    runs-on: [self-hosted, macOS]\n    steps:\n      - run: make\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 4)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "  mac:\n    if: "+publicOnly+"\n    runs-on: macos-latest\n")
	assert.Contains(t, got.Text, "  win:\n    if: (github.ref == 'refs/heads/master') && "+publicOnly+"\n    runs-on: windows-latest\n")
	assert.Contains(t, got.Text, "  wrapped:\n    if: ${{ (!cancelled()) && "+publicOnly+" }} # keep going\n")
	assert.Contains(t, got.Text, "  quoted:\n    if: (needs.plan.outputs.go == 'yes') && "+publicOnly+"\n")
	assert.Contains(t, got.Text, "  gated:\n    if: ${{ github.event.repository.visibility == 'public' && always() }}\n", "a gated job is not gated twice")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

func TestPrivateRunnerLeavesForeignJobsOfAnExemptRepository(t *testing.T) {
	body := workflowHeader() + "jobs:\n  mac:\n    runs-on: macos-latest\n    steps:\n      - run: make\n" +
		"  apple:\n    strategy:\n      matrix:\n        os: [ubuntu-latest, macos-14]\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n"
	for name, origin := range map[string]string{
		"exempt":        "https://github.com/wow-look-at-my/buildhost",
		"another owner": "https://github.com/someone/private-runner-fixture",
	} {
		path := runnerRepo(t, origin, body)
		assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner), name)
	}
}

func TestPrivateRunnerSplitsABlockMatrixList(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    strategy:\n      matrix:\n        os:\n          - ubuntu-22.04\n          - macos-latest # apple\n          - self-hosted-box\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 2)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "      matrix:\n        os: "+split(`["ubuntu-22.04","macos-latest","self-hosted-box"]`, `["ubuntu-22.04","self-hosted-box"]`)+"\n    runs-on: "+matrixVisible("os", `["ubuntu-22.04"]`, runnerSlim)+"\n")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}
