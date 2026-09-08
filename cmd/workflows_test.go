package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/gitmod/gitmodtest"
)

// runWorkflowsOn drives the command and returns what it printed. The command is
// built per call: tests run in parallel, and rootCmd's writer is shared.
func runWorkflowsOn(t *testing.T, paths ...string) (string, error) {
	t.Helper()
	return runWorkflowsOnly(t,paths...)
}

func runWorkflowsOnly(t *testing.T, only []string, paths ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := reportWorkflows(cmd, paths, only)
	return out.String(), err
}

const wall = "on: push\n\n# one\n# two\n# three\njobs: {}\n"

const atTheLimit = "on: push\n\n# one line is the limit\njobs: {}\n"

// The failure this command exists to prevent: a step placed before the checkout
// reads an empty workspace, and a pass there says the rule held when nothing
// was read at all.
func TestAWalkThatSelectsNothingFails(t *testing.T) {
	_, err := runWorkflowsOn(t,t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforced nothing")
	assert.Contains(t, err.Error(), "check the repository out")
}

func TestAWallOfCommentLinesIsReportedWithItsPlace(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, ".github/workflows/ci.yml", wall)

	out, err := runWorkflowsOn(t,dir)
	require.Error(t, err)
	assert.Contains(t, out, path)
	assert.Contains(t, out, "3 comment lines in a row")
	assert.Contains(t, out, "lines 3-5")
}

func TestABlockAtTheLimitPasses(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t,dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// .github is hidden, and the other walks in this package skip a hidden
// directory. Skipping it here drops every workflow and reads as a pass.
func TestTheWalkKeepsTheDotGithubDirectory(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", wall)

	_, err := runWorkflowsOn(t,dir)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "enforced nothing")
}

// An action manifest is judged wherever it sits, which is how the rule reaches
// a composite action a workflow calls with `uses: ./`.
func TestAnActionManifestOutsideDotGithubIsRead(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "my-action/action.yml", "name: a\nruns:\n  using: composite\n\n# one\n# two\n")

	out, err := runWorkflowsOn(t,dir)
	require.Error(t, err)
	assert.Contains(t, out, "2 comment lines in a row")
}

func TestTheWalkSkipsForeignAndBuildDirectories(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "node_modules/dep/action.yml", wall)
	writeAt(t, dir, "build/action.yml", wall)
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t,dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// A submodule carries its own CI, so the walk leaves its files to it. The
// control beside it is the caller's own workflow, which is still read: a skip
// that swallowed the whole walk would be indistinguishable from a rule turned
// off, and this is the only skip there is.
func TestASubmoduleIsNotRead(t *testing.T) {
	dir := gitmodtest.RepoWithSubmodule(t, "vendored")
	writeAt(t, dir, "vendored/.github/workflows/ci.yml", wall)
	writeAt(t, dir, ".github/workflows/ci.yml", atTheLimit)

	out, err := runWorkflowsOn(t, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
	assert.NotContains(t, out, "vendored")
}

// The forged exemption: .gitmodules names a directory that is not a gitlink, so
// a repository could declare its own source a submodule and stop being read.
func TestADeclaredPathThatIsNotAGitlinkFails(t *testing.T) {
	dir := gitmodtest.RepoWithFakeSubmodule(t, "src")
	writeAt(t, dir, "src/.github/workflows/ci.yml", wall)

	_, err := runWorkflowsOn(t, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gitlink")
}

// Naming a file IS the request, so its path does not have to look the part.
func TestANamedFileIsReadWhateverItsPath(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "somewhere.yml", wall)

	out, err := runWorkflowsOn(t,path)
	require.Error(t, err)
	assert.Contains(t, out, "3 comment lines in a row")
}

func TestAMissingWorkflowPathIsAnError(t *testing.T) {
	_, err := runWorkflowsOn(t,filepath.Join(t.TempDir(), "absent"))
	assert.Error(t, err)
}

// A caller that wants the comment-block rule must not pick up its siblings by
// asking for it, because a repository runs each of them from its own step.
func TestOnlyReportsTheRuleTheCallerNamed(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml",
		"on: push\n\n# one\n# two\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n")

	every, err := runWorkflowsOn(t,dir)
	require.Error(t, err)
	assert.Contains(t, every, "yaml/comment-block")
	assert.Contains(t, every, "yaml/all-builds-job")

	narrowed, err := runWorkflowsOnly(t,[]string{"yaml/comment-block"}, dir)
	require.Error(t, err)
	assert.Contains(t, narrowed, "yaml/comment-block")
	assert.NotContains(t, narrowed, "yaml/all-builds-job")
}

// A rule the caller named that no findings match is a pass, not a walk failure.
func TestOnlyPassesWhenTheNamedRuleFindsNothing(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml",
		"on: push\n\njobs:\n  all-builds:\n    runs-on: ubuntu-latest\n")

	out, err := runWorkflowsOnly(t,[]string{"yaml/comment-block"}, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "1 file(s) read")
}

// A typo that quietly reports nothing reads exactly like a clean tree.
func TestAnUnknownRuleNameIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, ".github/workflows/ci.yml", wall)

	_, err := runWorkflowsOnly(t,[]string{"yaml/nosuch"}, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown rule")
	assert.Contains(t, err.Error(), "yaml/comment-block")
}
