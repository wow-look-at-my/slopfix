package slopfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// guardedRunScripts carries shell guards from sglang's fork-ci.yml. Each
// annotates and exits inside an if, so the test-in-workflow rule reads it.
const guardedRunScripts = "name: CI\n" +
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
	"      - run: |\n" +
	"          shopt -s nullglob\n" +
	"          WHEELS=(dist/*.whl)\n" +
	"          if [ ${#WHEELS[@]} -eq 0 ]; then\n" +
	"            echo \"::error::no sglang-kernel wheel was built\"; exit 1\n" +
	"          fi\n" +
	"      - run: |\n" +
	"          for TAG in a b; do\n" +
	"            SOURCES=()\n" +
	"            if [ ${#SOURCES[@]} -eq 0 ]; then\n" +
	"              echo \"::error::no per-arch image was pushed for ${TAG}\"; exit 1\n" +
	"            fi\n" +
	"          done\n"

const guardedRunScriptsPath = ".github/workflows/fork-ci.yml"

// The fixture provokes the rule, and the rule only warns: a run: script line
// is shell, and no rewrite deletes it.
func TestATestInARunScriptIsAWarning(t *testing.T) {
	var hits []string
	for _, f := range slopfix.CheckContent(guardedRunScriptsPath, guardedRunScripts) {
		require.True(t, f.Warning(), "only the warning rule may read this fixture: %s", f)
		if f.ID == workflow.IDTestInYAML {
			hits = append(hits, f.Detail)
		}
	}
	assert.Len(t, hits, 2)
	assert.True(t, slopfix.WarningIDs.Contains(workflow.IDTestInYAML))
	assert.False(t, slopfix.Repairable(workflow.IDTestInYAML))
}

func TestFixKeepsEveryLineOfARunScript(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: guardedRunScripts, Path: guardedRunScriptsPath, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.False(t, repair.Changed)
	assert.Equal(t, guardedRunScripts, repair.Text)
	assert.Empty(t, repair.Removed)

	yaml := workflow.Fix(guardedRunScripts, func(string) bool { return true })
	assert.False(t, yaml.Changed)
	assert.Equal(t, guardedRunScripts, yaml.Text)
}

// The same through the file `slopfix fix` rewrites: the file is untouched.
func TestFixingAWorkflowFileKeepsItsRunScripts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "fork-ci.yml")
	require.NoError(t, os.WriteFile(path, []byte(guardedRunScripts), 0o644))

	repair, err := slopfix.FixFileWith(path, slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
	require.NoError(t, err)
	assert.False(t, repair.Changed)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, guardedRunScripts, string(after))
}
