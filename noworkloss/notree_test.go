package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ~/Downloads is what these cover: an ordinary place to unpack an archive,
// with no .git entry in any ancestor. Each case pairs an allowance with the
// same command inside a real work tree, so a rule that stopped answering at
// all would fail the denial half.

// plainDir has no .git entry above it and no project directory naming it.
func plainDir(t *testing.T) string {
	t.Helper()
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "archive.zst"), []byte("payload\n"), 0o644))
	require.Empty(t, repoRoot(dir), "the fixture must sit outside every work tree")
	return dir
}

// split names the chunks itself, so the directory is what gets judged. Write,
// which the refusal named as the repair, cannot author a binary chunk.
func TestAllowsSplitOutsideEveryWorkTree(t *testing.T) {
	dir := plainDir(t)
	assert.Empty(t, ask(t, dir, "split -b 14m archive.zst archive.zst.part-"))
}

func TestDeniesSplitInsideTheWorkTree(t *testing.T) {
	root := newTree(t)
	r := ask(t, root, "split -b 14m src.txt src.txt.part-")
	assert.Contains(t, r, "working tree")
}

// mv into a directory overwrites whatever the sources are named, and the
// names came from a glob. An overwrite costs what git no longer holds.
func TestAllowsMoveWithUnresolvableSourcesOutsideEveryWorkTree(t *testing.T) {
	dir := plainDir(t)
	assert.Empty(t, ask(t, dir, `mv "$TMPDIR/zstsplit/"*.part-* `+dir+"/"))
}

func TestDeniesMoveWithUnresolvableSourcesIntoTheWorkTree(t *testing.T) {
	dir := newRepo(t)
	r := denied(t, dir, `mv "$TMPDIR/zstsplit/"*.part-* `+dir+"/")
	assert.Contains(t, r, "cannot resolve")
}

// An untracked root still stands in for a repository under it. Dropping such
// a root outright let `cd <repo> && git reset --hard` through from above.
func TestGuardsARepositoryBelowAnUntrackedRoot(t *testing.T) {
	dir := plainDir(t)
	root := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src.txt"), []byte("original\n"), 0o644))
	r := ask(t, dir, "split -b 14m archive.zst repo/src.txt.part-")
	assert.Contains(t, r, "working tree")
}

// Scope is the work tree, not the session's directory, so the allowance above
// cannot be reached by running the command from somewhere else.
func TestGuardsTheProjectDirectoryFromOutsideIt(t *testing.T) {
	root := newTree(t)
	dir := plainDir(t)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	r := ask(t, dir, "split -b 14m archive.zst "+root+"/src.txt.part-")
	assert.Contains(t, r, "working tree")
}
