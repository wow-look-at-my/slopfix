package slopfix

import _ "embed"

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
		testPrivateRunnerRewritesEachHostedLinuxJobYml +
		testPrivateRunnerRewritesEachHostedLinuxJobYml2 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml3 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml4 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml5 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml6 +
		"  flow: {runs-on: \"ubuntu-\\\n      latest\", steps: [{run: make}]}\n" +
		testPrivateRunnerRewritesEachHostedLinuxJobYml7 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml8 +
		testPrivateRunnerRewritesEachHostedLinuxJobYml9 +
		"  done:\n    runs-on: " + visible("ubuntu-latest", "CI_RUNNER") + "\n    steps:\n      - run: make\n" +
		testPrivateRunnerRewritesEachHostedLinuxJobYml10
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
	body := workflowHeader() + testPrivateRunnerReadsNoVisibilityYml
	path := runnerRepo(t, "https://github.com/wow-look-at-my/public-runner-fixture", body)
	event := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(event, []byte(`{"repository":{"full_name":"wow-look-at-my/public-runner-fixture","private":false,"visibility":"public"}}`), 0o644))
	t.Setenv("GITHUB_EVENT_PATH", event)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 1, "a public repository gets the expression too")
}

func TestPrivateRunnerSkipsRepositoriesItDoesNotGovern(t *testing.T) {
	body := workflowHeader() + testPrivateRunnerSkipsRepositoriesItDoesNotGovernYml
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
		body := workflowHeader() + testPrivateRunnerSendsAContainerStepToTheDockerRunnerYml + step.uses + "\n"
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
	body := workflowHeader() + testPrivateRunnerChoosesTheDockerRunnerForAnUnreadActionYml
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
		body := workflowHeader() + testPrivateRunnerMovesAJobThatNeedsTheHostYml + steps
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
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml +
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml2 +
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml3 +
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml4 +
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml5 +
		testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml6 + done + "\n    steps:\n      - run: make\n" +
		"  flow: {strategy: {matrix: {os: [ubuntu-latest, windows-latest]}}, runs-on: '${{ matrix.os }}', steps: [{run: make}]}\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 7)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: "+matrixVisible("os", `["ubuntu-22.04","ubuntu-24.04"]`, runnerSlim)+"\n", "the arm values stay out of the list")
	assert.Contains(t, got.Text, "        include: "+split(
		`[{"os":"ubuntu-22.04","arch":"amd64"},{"os":"ubuntu-24.04","arch":"amd64"},{"os":"ubuntu-22.04-arm","arch":"arm64"}]`,
		`[{"os":"ubuntu-22.04","arch":"amd64"},{"os":"ubuntu-24.04","arch":"amd64"}]`)+"\n    runs-on:", "a private repository drops the arm include entry")
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
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml +
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml2 +
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml3 +
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml4 +
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml5 +
		testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml6
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
	body := workflowHeader() + testPrivateRunnerLeavesForeignJobsOfAnExemptRepositoryYml +
		testPrivateRunnerLeavesForeignJobsOfAnExemptRepositoryYml2
	for name, origin := range map[string]string{
		"exempt":        "https://github.com/wow-look-at-my/buildhost",
		"another owner": "https://github.com/someone/private-runner-fixture",
	} {
		path := runnerRepo(t, origin, body)
		assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner), name)
	}
}

func TestPrivateRunnerSplitsAnIncludeList(t *testing.T) {
	body := workflowHeader() + "jobs:\n" +
		testPrivateRunnerSplitsAnIncludeListYml +
		testPrivateRunnerSplitsAnIncludeListYml2 +
		testPrivateRunnerSplitsAnIncludeListYml3
	path := runnerRepo(t, PrivateRunnerFixture, body)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "        os: "+split(`["ubuntu-latest","macos-14"]`, `["ubuntu-latest"]`)+"\n        include: "+split(
		`[{"os":"windows-2022","flags":{"cc":"it''s","n":2,"on":true} },{"os":"ubuntu-latest","extra":["1.20",3]}]`,
		`[{"os":"ubuntu-latest","extra":["1.20",3]}]`)+"\n        # the end\n", "the list and the include both drop their foreign values")
	assert.Contains(t, got.Text, "          - os: macos-15\n    if: "+publicOnly+"\n", "an include of foreign legs alone keeps the job to a public repository")
	assert.Contains(t, got.Text, "          - &win {os: windows-2022}\n", "an anchored entry is not read")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

func TestPrivateRunnerReadsTheMatrixKeyOfAWrittenExpression(t *testing.T) {
	written := "${{ matrix.os == 'ubuntu-latest' && (vars.CI_RUNNER_DIND || 'ubuntu-latest') || matrix.os }}"
	prefix := "${{ startsWith(matrix.os, 'ubuntu') && (vars.CI_RUNNER || matrix.os) || matrix.os }}"
	body := workflowHeader() + "jobs:\n" +
		testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml + written + "\n    steps:\n      - run: make\n" +
		testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml2 + prefix + "\n    steps:\n      - run: make\n" +
		testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml3 +
		testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml4
	path := runnerRepo(t, PrivateRunnerFixture, body)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "        os: "+split(`["ubuntu-latest","macos-latest","windows-latest"]`, `["ubuntu-latest"]`)+"\n    runs-on: "+written+"\n", "a written expression keeps its Linux routing")
	assert.Contains(t, got.Text, "        include: "+split(`[{"name":"Linux","os":"ubuntu-24.04"},{"name":"Mac","os":"macos-14"}]`, `[{"name":"Linux","os":"ubuntu-24.04"}]`)+"\n    runs-on: "+prefix+"\n")
	assert.Contains(t, got.Text, "  mac:\n    if: "+publicOnly+"\n    runs-on: macOS-latest\n", "a label in any case is the same runner")
	assert.Contains(t, got.Text, "        os: [ubuntu-latest, macos-14]\n", "an expression that reads two matrix keys is not read")
}

func TestPrivateRunnerSplitsABlockMatrixList(t *testing.T) {
	body := workflowHeader() + testPrivateRunnerSplitsABlockMatrixListYml
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 2)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "      matrix:\n        os: "+split(`["ubuntu-22.04","macos-latest","self-hosted-box"]`, `["ubuntu-22.04","self-hosted-box"]`)+"\n    runs-on: "+matrixVisible("os", `["ubuntu-22.04"]`, runnerSlim)+"\n")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-2.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml2 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-3.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml3 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-4.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml4 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-5.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml5 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-6.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml6 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-7.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml7 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-8.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml8 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-9.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml9 string

//go:embed testdata/test-private-runner-rewrites-each-hosted-linux-job-10.yml
var testPrivateRunnerRewritesEachHostedLinuxJobYml10 string

//go:embed testdata/test-private-runner-reads-no-visibility.yml
var testPrivateRunnerReadsNoVisibilityYml string

//go:embed testdata/test-private-runner-skips-repositories-it-does-not-govern.yml
var testPrivateRunnerSkipsRepositoriesItDoesNotGovernYml string

//go:embed testdata/test-private-runner-sends-a-container-step-to-the-docker-runner.yml
var testPrivateRunnerSendsAContainerStepToTheDockerRunnerYml string

//go:embed testdata/test-private-runner-chooses-the-docker-runner-for-an-unread-action.yml
var testPrivateRunnerChoosesTheDockerRunnerForAnUnreadActionYml string

//go:embed testdata/test-private-runner-moves-a-job-that-needs-the-host.yml
var testPrivateRunnerMovesAJobThatNeedsTheHostYml string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix-2.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml2 string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix-3.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml3 string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix-4.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml4 string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix-5.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml5 string

//go:embed testdata/test-private-runner-routes-the-hosted-linux-values-of-a-matrix-6.yml
var testPrivateRunnerRoutesTheHostedLinuxValuesOfAMatrixYml6 string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository-2.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml2 string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository-3.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml3 string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository-4.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml4 string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository-5.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml5 string

//go:embed testdata/test-private-runner-keeps-a-foreign-job-to-a-public-repository-6.yml
var testPrivateRunnerKeepsAForeignJobToAPublicRepositoryYml6 string

//go:embed testdata/test-private-runner-leaves-foreign-jobs-of-an-exempt-repository.yml
var testPrivateRunnerLeavesForeignJobsOfAnExemptRepositoryYml string

//go:embed testdata/test-private-runner-leaves-foreign-jobs-of-an-exempt-repository-2.yml
var testPrivateRunnerLeavesForeignJobsOfAnExemptRepositoryYml2 string

//go:embed testdata/test-private-runner-splits-an-include-list.yml
var testPrivateRunnerSplitsAnIncludeListYml string

//go:embed testdata/test-private-runner-splits-an-include-list-2.yml
var testPrivateRunnerSplitsAnIncludeListYml2 string

//go:embed testdata/test-private-runner-splits-an-include-list-3.yml
var testPrivateRunnerSplitsAnIncludeListYml3 string

//go:embed testdata/test-private-runner-reads-the-matrix-key-of-a-written-expression.yml
var testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml string

//go:embed testdata/test-private-runner-reads-the-matrix-key-of-a-written-expression-2.yml
var testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml2 string

//go:embed testdata/test-private-runner-reads-the-matrix-key-of-a-written-expression-3.yml
var testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml3 string

//go:embed testdata/test-private-runner-reads-the-matrix-key-of-a-written-expression-4.yml
var testPrivateRunnerReadsTheMatrixKeyOfAWrittenExpressionYml4 string

//go:embed testdata/test-private-runner-splits-a-block-matrix-list.yml
var testPrivateRunnerSplitsABlockMatrixListYml string
