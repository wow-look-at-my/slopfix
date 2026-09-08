package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A directory under no version control is not this hook's business. Both
// halves say so already -- the destruction half in judge, the language server
// in scope.ts -- and the provenance half did not. ~/Downloads is the case
// these cover: an ordinary place to unpack an archive, with no .git entry in
// any ancestor.
//
// Each case is a pair. The denial half runs the same command inside a real
// working tree, so a rule that stopped answering at all would pass the
// allowance and fail the denial.

// plainDir is a directory with no .git entry above it and no project
// directory naming it.
func plainDir(t *testing.T) string {
	t.Helper()
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "archive.zst"), []byte("payload\n"), 0o644))
	require.Empty(t, repoRoot(dir), "the fixture must sit outside every work tree")
	return dir
}

// The reported incident. split writes chunks whose names it invents, so the
// provenance half judges the directory rather than a path. Outside a work
// tree there is no version to arrive before the write, and the repair the
// denial named -- Write -- cannot author a binary chunk at all.
func TestAllowsSplitOutsideEveryWorkTree(t *testing.T) {
	dir := plainDir(t)
	assert.Empty(t, ask(t, dir, "split -b 14m archive.zst archive.zst.part-"))
}

func TestDeniesSplitInsideTheWorkTree(t *testing.T) {
	root := newTree(t)
	r := ask(t, root, "split -b 14m src.txt src.txt.part-")
	assert.Contains(t, r, "working tree")
}

// The second half of the same incident. mv into a directory overwrites
// whatever the sources are named, and the names came from a glob. What an
// overwrite costs is what git no longer holds, so outside a work tree it
// costs nothing, and `git add -A && git commit` is not a way out of a
// directory with no repository in it.
func TestAllowsMoveWithUnresolvableSourcesOutsideEveryWorkTree(t *testing.T) {
	dir := plainDir(t)
	assert.Empty(t, ask(t, dir, `mv "$TMPDIR/zstsplit/"*.part-* `+dir+"/"))
}

func TestDeniesMoveWithUnresolvableSourcesIntoTheWorkTree(t *testing.T) {
	dir := newRepo(t)
	r := denied(t, dir, `mv "$TMPDIR/zstsplit/"*.part-* `+dir+"/")
	assert.Contains(t, r, "cannot resolve")
}

// A root outside every work tree still stands in for a repository UNDER it.
// The first version of this fix dropped such a root entirely, which let
// `cd <repo> && git reset --hard` through from the directory above.
func TestGuardsARepositoryBelowAnUntrackedRoot(t *testing.T) {
	dir := plainDir(t)
	root := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src.txt"), []byte("original\n"), 0o644))
	r := ask(t, dir, "split -b 14m archive.zst repo/src.txt.part-")
	assert.Contains(t, r, "working tree")
}

// The scope is the work tree, not the session's own directory. A write into
// a repository the session is not rooted on is still refused, so the rule
// above cannot be reached by running the command from somewhere else.
func TestGuardsTheProjectDirectoryFromOutsideIt(t *testing.T) {
	root := newTree(t)
	dir := plainDir(t)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	r := ask(t, dir, "split -b 14m archive.zst "+root+"/src.txt.part-")
	assert.Contains(t, r, "working tree")
}
