package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runScriptGuards are shell guards from sglang's fork-ci.yml. Each line
// annotates and exits inside an if, which the test-in-workflow rule reads.
var runScriptGuards = []struct{ open, body, close string }{
	{
		open:  "          if [ ${#WHEELS[@]} -eq 0 ]; then\n",
		body:  "            echo \"::error::no sglang-kernel wheel was built\"; exit 1\n",
		close: "          fi\n",
	},
	{
		open:  "          if [ ${#SOURCES[@]} -eq 0 ]; then\n",
		body:  "            echo \"::error::no per-arch image was pushed for ${TAG}\"; exit 1\n",
		close: "          fi\n",
	},
}

const runScriptHead = "name: CI\n" +
	"on:\n" +
	"  push:\n" +
	"    branches: ['**']\n" +
	"concurrency:\n" +
	"  group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n" +
	"  cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n" +
	"jobs:\n" +
	"  kernel:\n" +
	"    runs-on: ubuntu-latest\n" +
	"    steps:\n" +
	"      - run: |\n"

// runScriptFile writes content under a workflows directory, where the yaml
// rules read it, and answers its path.
func runScriptFile(t *testing.T, content string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "fork-ci.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// An Edit that puts a guard's body back between its if and fi lands as
// written. The hook rewrote it and cut the line, which left bash a then with
// nothing before fi.
func TestTheHookKeepsALineAnEditAddsToARunScript(t *testing.T) {
	for _, g := range runScriptGuards {
		t.Run(g.body, func(t *testing.T) {
			path := runScriptFile(t, runScriptHead+g.open+g.close)
			payload := map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Edit",
				"tool_input": map[string]any{
					"file_path":  path,
					"old_string": g.open + g.close,
					"new_string": g.open + g.body + g.close,
				},
			}
			got := ask(t, payload)

			assert.NotContains(t, got.body, "permissionDecision")
			assert.Nil(t, got.out["updatedInput"], "the hook rewrote the edit: %s", got.body)
		})
	}
}

// A Write of the whole workflow lands unchanged too.
func TestTheHookKeepsEveryLineAWriteGivesARunScript(t *testing.T) {
	content := runScriptHead
	for _, g := range runScriptGuards {
		content += g.open + g.body + g.close
	}
	got := ask(t, write(filepath.Join(t.TempDir(), ".github", "workflows", "fork-ci.yml"), content))

	assert.NotContains(t, got.body, "permissionDecision")
	assert.Nil(t, got.out["updatedInput"], "the hook rewrote the write: %s", got.body)
}
