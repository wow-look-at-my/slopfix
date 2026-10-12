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
		"  done:\n    runs-on: " + visible("ubuntu-latest", "CI_RUNNER") + "\n    steps:\n      - run: make\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	assert.Len(t, only(CheckContent(path, body), workflow.IDPrivateRunner), 7)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
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
