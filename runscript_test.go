package slopfix_test

import (
	"os"
	"path/filepath"
	"strings"
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

// The fixture provokes the rule, and the rule only warns.
func TestATestInARunScriptIsAWarning(t *testing.T) {
	var hits []string
	for _, f := range slopfix.CheckContent(guardedRunScriptsPath, guardedRunScripts) {
		require.True(t, f.Warning(), "only the warning rule may read this fixture: %s", f)
		if f.ID == workflow.IDTestInYAML {
			hits = append(hits, f.Detail)
		}
	}
	assert.Len(t, hits, 2)
	assert.True(t, workflow.WarningIDs.Contains(workflow.IDTestInYAML))
	assert.True(t, slopfix.Repairable(workflow.IDTestInYAML))
}

// The scripts guardedRunScripts moves out, by their path from the checkout root.
var guardedScripts = map[string]string{
	".github/scripts/fork-ci-kernel-1.sh": "#!/usr/bin/env bash\nset -eo pipefail\n" +
		"shopt -s nullglob\n" +
		"WHEELS=(dist/*.whl)\n" +
		"if [ ${#WHEELS[@]} -eq 0 ]; then\n" +
		"  echo \"::error::no sglang-kernel wheel was built\"; exit 1\n" +
		"fi\n",
	".github/scripts/fork-ci-kernel-2.sh": "#!/usr/bin/env bash\nset -eo pipefail\n" +
		"for TAG in a b; do\n" +
		"  SOURCES=()\n" +
		"  if [ ${#SOURCES[@]} -eq 0 ]; then\n" +
		"    echo \"::error::no per-arch image was pushed for ${TAG}\"; exit 1\n" +
		"  fi\n" +
		"done\n",
}

// guardedRunScriptsFixed is guardedRunScripts with each step running its file.
var guardedRunScriptsFixed = guardedRunScripts[:strings.Index(guardedRunScripts, "      - run: |\n")] +
	"      - run: bash .github/scripts/fork-ci-kernel-1.sh\n" +
	"      - run: bash .github/scripts/fork-ci-kernel-2.sh\n"

// The repair cuts no line of a run: script. Each script moves whole into a
// file, and its step runs the file.
func TestFixMovesEveryLineOfARunScriptIntoAFile(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: guardedRunScripts, Path: guardedRunScriptsPath, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.Equal(t, guardedRunScriptsFixed, repair.Text)
	assert.Empty(t, repair.Removed)
	created := map[string]string{}
	for _, c := range repair.Created {
		created[filepath.ToSlash(c.Path)] = c.Text
	}
	assert.Equal(t, guardedScripts, created)

	yaml := workflow.Fix(guardedRunScripts, func(string) bool { return true })
	assert.False(t, yaml.Changed, "a caller that writes the workflow alone gets no change")
	assert.Equal(t, guardedRunScripts, yaml.Text)
}

// The same through the file `slopfix fix` rewrites: the scripts land beside it.
func TestFixingAWorkflowFileWritesItsRunScripts(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "fork-ci.yml")
	require.NoError(t, os.WriteFile(path, []byte(guardedRunScripts), 0o644))

	repair, err := slopfix.FixFileWith(path, slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
	require.NoError(t, err)
	assert.True(t, repair.Changed)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, guardedRunScriptsFixed, string(after))
	for rel, want := range guardedScripts {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}
}
