package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runWorkflowsOn drives the command and returns what it printed. The command is
// built per call: tests run in parallel, and rootCmd's writer is shared.
func runWorkflowsOn(t *testing.T, excludes []string, paths ...string) (string, error) {
	t.Helper()
	previous := workflowExcludes
	workflowExcludes = excludes
	t.Cleanup(func() { workflowExcludes = previous })

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runWorkflows(cmd, paths)
	return out.String(), err
}

const wall = "on: push\n\n# one\n# two\n# three\njobs: {}\n"

const atTheLimit = "on: push\n\n# one line is the limit\njobs: {}\n"

// The failure this command exists to prevent: a step placed before the checkout
// reads an empty workspace, and a pass there says the rule held when nothing
// was read at all.
func TestAWalkThatSelectsNothingFails(t *testing.T) {
	_, err := runWorkflowsOn(t, nil, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforced nothing")
	assert.Contains(t, err.Error(), "check the repository out")
}

func TestAWallOfCommentLinesIsReportedWithItsPlace(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, ".github/workflows/ci.yml", wall)

	out, err := runWorkflowsOn(t, nil, dir)
	require.Error(t, err)
	assert.Contains(t, out, path)
	assert.Contains(t, out, "3 comment lines in a row")
	assert.Contains(t, out, "lines 3-5")
}

func TestABlockAtTheLimitPasses(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t, nil, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// .github is hidden, and the other walks in this package skip a hidden
// directory. Skipping it here drops every workflow and reads as a pass.
func TestTheWalkKeepsTheDotGithubDirectory(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", wall)

	_, err := runWorkflowsOn(t, nil, dir)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "enforced nothing")
}

// An action manifest is judged wherever it sits, which is how the rule reaches
// a composite action a workflow calls with `uses: ./`.
func TestAnActionManifestOutsideDotGithubIsRead(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "my-action/action.yml", "name: a\nruns:\n  using: composite\n\n# one\n# two\n")

	out, err := runWorkflowsOn(t, nil, dir)
	require.Error(t, err)
	assert.Contains(t, out, "2 comment lines in a row")
}

func TestTheWalkSkipsForeignAndBuildDirectories(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "node_modules/dep/action.yml", wall)
	writeAt(t, dir, "build/action.yml", wall)
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t, nil, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// A fixture breaks the rule on purpose, so the run that reads it needs a way
// past it that is not switching the rule off.
func TestAnExcludedFixtureIsNotRead(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "test/fixtures/wall/.github/workflows/ci.yml", wall)
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t, []string{"**/test/fixtures/**"}, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// An exclude that swallows the whole walk still fails, because the run that
// read nothing enforced nothing.
func TestAnExcludeThatSelectsNothingStillFails(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	_, err := runWorkflowsOn(t, []string{"**"}, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforced nothing")
}

// Naming a file IS the request, so its path does not have to look the part.
func TestANamedFileIsReadWhateverItsPath(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "somewhere.yml", wall)

	out, err := runWorkflowsOn(t, nil, path)
	require.Error(t, err)
	assert.Contains(t, out, "3 comment lines in a row")
}

func TestAMissingWorkflowPathIsAnError(t *testing.T) {
	_, err := runWorkflowsOn(t, nil, filepath.Join(t.TempDir(), "absent"))
	assert.Error(t, err)
}

func TestAGlobStarDoesNotCrossASeparator(t *testing.T) {
	matches, err := excludeMatcher([]string{"a/*/action.yml"})
	require.NoError(t, err)
	assert.True(t, matches("a/b/action.yml"))
	assert.False(t, matches("a/b/c/action.yml"))
}
