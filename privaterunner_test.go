package slopfix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// The event payload says the fixture repository is private, the way an Actions run reads it.
func init() {
	dir, err := os.MkdirTemp("", "slopfix-event-")
	if err != nil {
		panic(err)
	}
	event := filepath.Join(dir, "event.json")
	payload := `{"repository":{"full_name":"wow-look-at-my/private-runner-fixture","private":true}}`
	if err := os.WriteFile(event, []byte(payload), 0o644); err != nil {
		panic(err)
	}
	workflow.Resolver = forkscope.Resolver{Getenv: func(key string) string {
		if key == "GITHUB_EVENT_PATH" {
			return event
		}
		return os.Getenv(key)
	}}
}

// runnerRepo writes a workflow into a repository with origin, and answers its path.
func runnerRepo(t *testing.T, origin, body string) string {
	t.Helper()
	c, err := Materialize(t.TempDir(), RuleCase{Name: "runner", Path: ".github/workflows/ci.yml", Text: body, Origin: origin})
	require.NoError(t, err)
	return filepath.Join(c.Root, ".github", "workflows", "ci.yml")
}

func TestPrivateRunnerRewritesEachHostedLinuxJob(t *testing.T) {
	body := workflowHeader() + "jobs:\n" +
		"  build:\n    runs-on: ubuntu-24.04 # pinned\n    steps:\n      - run: make\n" +
		"  image:\n    runs-on: 'ubuntu-latest'\n    services:\n      db:\n        image: postgres\n    steps:\n      - run: make\n" +
		"  mac:\n    runs-on: macos-latest\n    steps:\n      - run: make\n" +
		"  matrix:\n    runs-on: ${{ matrix.os }}\n    steps:\n      - run: make\n"
	path := runnerRepo(t, PrivateRunnerFixture, body)
	got, err := FixFileWith(path, Request{IDs: []string{workflow.IDPrivateRunner}})
	require.NoError(t, err)
	assert.Contains(t, got.Text, "runs-on: ${{ vars.CI_RUNNER || 'ubuntu-24.04' }} # pinned\n")
	assert.Contains(t, got.Text, "runs-on: ${{ vars.CI_RUNNER_DIND || 'ubuntu-latest' }}\n")
	assert.Contains(t, got.Text, "runs-on: macos-latest\n", "the org runners are Linux")
	assert.Contains(t, got.Text, "runs-on: ${{ matrix.os }}\n", "an expression is not read")
	assert.Empty(t, only(CheckContent(path, got.Text), workflow.IDPrivateRunner))
}

func TestPrivateRunnerSkipsRepositoriesItDoesNotGovern(t *testing.T) {
	body := workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n"
	for name, origin := range map[string]string{
		"no origin":       "",
		"another owner":   "https://github.com/someone/private-runner-fixture",
		"exempt":          "https://github.com/wow-look-at-my/infra",
		"another git host": "https://example.com/wow-look-at-my/private-runner-fixture",
	} {
		path := runnerRepo(t, origin, body)
		assert.Empty(t, only(CheckContent(path, body), workflow.IDPrivateRunner), name)
	}
}
