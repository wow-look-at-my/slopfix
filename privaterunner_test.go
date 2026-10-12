package slopfix

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// runnerRepo writes a workflow into a private repository with origin, and answers its path.
func runnerRepo(t *testing.T, origin, body string) string {
	t.Helper()
	c, err := Materialize(t.TempDir(), RuleCase{Name: "runner", Path: ".github/workflows/ci.yml", Text: body, Origin: origin, Private: origin != ""})
	require.NoError(t, err)
	return filepath.Join(c.Root, ".github", "workflows", "ci.yml")
}

func TestPrivateRunnerLeavesAPublicRepositoryAlone(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n"
	c, err := Materialize(t.TempDir(), RuleCase{Name: "public", Path: ".github/workflows/ci.yml", Text: body, Origin: "https://github.com/wow-look-at-my/public-runner-fixture"})
	require.NoError(t, err)
	path := filepath.Join(c.Root, ".github", "workflows", "ci.yml")
	assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner))
}

func TestPrivateRunnerRewritesEachHostedLinuxJob(t *testing.T) {
	body := workflowHeader() + "jobs:\n" +
		"  build:\n    runs-on: ubuntu-24.04 # pinned\n    steps:\n      - run: make\n" +
		"  image:\n    runs-on: 'ubuntu-latest'\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n" +
		"  mac:\n    runs-on: macos-latest\n    steps:\n      - run: make\n" +
		"  matrix:\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n" +
		"  push:\n    runs-on: ubuntu-22.04\n    steps:\n      - uses: wow-look-at-my/actions@ghcr#latest\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }} # pinned\n")
	assert.Contains(t, got.Text, "runs-on: ${{ vars.CI_RUNNER_DIND || 'ubuntu-latest' }}\n")
	assert.Contains(t, got.Text, "runs-on: ${{ vars.CI_RUNNER_DIND || 'ubuntu-22.04' }}\n", "the ghcr action builds with docker")
	assert.Contains(t, got.Text, "runs-on: macos-latest\n", "the org runners are Linux")
	assert.Contains(t, got.Text, "runs-on: ${{ matrix.os }}\n", "an expression is not read")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
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
		c := RuleCase{Name: name, Path: ".github/workflows/ci.yml", Text: body, Origin: PrivateRunnerFixture, Private: true}
		if step.using != "" {
			c.Actions = map[string]string{step.uses: step.using}
		}
		c, err := Materialize(t.TempDir(), c)
		require.NoError(t, err)
		path := filepath.Join(c.Root, ".github", "workflows", "ci.yml")
		got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
		require.NoError(t, err)
		assert.Contains(t, got.Text, "runs-on: ${{ "+step.want+" || 'ubuntu-latest' }}\n", name)
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
	assert.Contains(t, got.Text, "runs-on: ${{ "+runnerDind+" || 'ubuntu-latest' }}\n")
}

// The org runner variables, as the repair writes them.
const (
	runnerSlim = "vars.CI_RUNNER"
	runnerDind = "vars.CI_RUNNER_DIND"
)
