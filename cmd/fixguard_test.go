package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guardBody holds a finding fix repairs, so a run that got past the guard would change it.
const guardBody = "# Title\n\nIt doesn't hold the lock.\n"

// fixMode sets the repair flag for one test and restores it.
func fixMode(t *testing.T, fix bool) {
	t.Helper()
	old := checkFix
	checkFix = fix
	t.Cleanup(func() { checkFix = old })
}

// A directory in no work tree, such as one holding many repositories, gets no
// repair at all: fix refuses, names the directory, and writes nothing.
func TestFixOutsideAWorkTreeRefusesAndWritesNothing(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	put(t, dir, "README.md", guardBody)
	inner := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(inner, 0o755))
	gitRun(t, inner, "init", "-q")
	put(t, inner, "README.md", guardBody)
	t.Chdir(dir)
	fixMode(t, true)

	err := runCheck(rootCmd, []string{"."})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git work tree")
	resolved, rerr := filepath.EvalSymlinks(dir)
	require.NoError(t, rerr)
	assert.True(t, containsEither(err.Error(), dir, resolved), "the error names the directory: %s", err)
	assert.Equal(t, guardBody, readT(t, filepath.Join(dir, "README.md")))
	assert.Equal(t, guardBody, readT(t, filepath.Join(inner, "README.md")))
}

// A path outside the work tree the run starts in is refused, and stays as written.
func TestFixWithAPathOutsideTheWorkTreeRefuses(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	other := t.TempDir()
	put(t, other, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, true)

	err := runCheck(rootCmd, []string{filepath.Join(other, "README.md")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside")
	assert.Equal(t, guardBody, readT(t, filepath.Join(other, "README.md")))
}

// A nested repository inside the work tree is a tree of its own, so a path
// in it is refused.
func TestFixWithAPathInANestedRepositoryRefuses(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	nested := filepath.Join(repo, "vendor", "lib")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	gitRun(t, nested, "init", "-q")
	put(t, nested, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, true)

	err := runCheck(rootCmd, []string{filepath.Join(nested, "README.md")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "belongs to the work tree")
	assert.Equal(t, guardBody, readT(t, filepath.Join(nested, "README.md")))
}

// A relative path inside the work tree the run starts in is repaired.
func TestFixInsideTheWorkTreeRepairs(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	put(t, repo, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, true)

	require.NoError(t, runCheck(rootCmd, []string{"README.md"}))
	assert.NotContains(t, readT(t, filepath.Join(repo, "README.md")), "doesn't")
}

// check only reads, so it runs in a directory that is in no work tree.
func TestCheckOutsideAWorkTreeStillReports(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	put(t, dir, "README.md", guardBody)
	t.Chdir(dir)
	fixMode(t, false)

	assert.ErrorIs(t, runCheck(rootCmd, []string{"README.md"}), errFindings)
	assert.Equal(t, guardBody, readT(t, filepath.Join(dir, "README.md")))
}

// containsEither reports whether s names either spelling of a directory.
func containsEither(s, a, b string) bool {
	return strings.Contains(s, a) || strings.Contains(s, b)
}
