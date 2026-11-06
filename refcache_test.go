package slopfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/gitmod"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// git runs a git command in dir and fails the test on an error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := gitmod.Command(dir, append([]string{"-c", "user.name=slopfix", "-c", "user.email=slopfix@localhost", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// In a linked work tree .git is a file. A tree run reads the ref cache from
// the git directory the work trees share, and the API is not asked.
func TestATreeRunInALinkedWorkTreeReadsTheSharedRefCache(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "tok")

	main := t.TempDir()
	dir := filepath.Join(main, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci.yml"), []byte(cachedTreeYml), 0o644))
	git(t, main, "init", "-q")
	git(t, main, "add", "-A")
	git(t, main, "commit", "-q", "-m", "fixture")
	require.NoError(t, os.WriteFile(filepath.Join(main, ".git", "slopfix-refs.json"), []byte(`{"o/r":"master","o/r@feature":"tag"}`), 0o644))

	linked := filepath.Join(t.TempDir(), "linked")
	git(t, main, "worktree", "add", "-q", "--detach", linked)
	info, err := os.Lstat(filepath.Join(linked, ".git"))
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "a linked work tree's .git is a file")

	tree := slopfix.CheckTreeWith(linked, slopfix.Request{IDs: []string{workflow.IDBranchPin}})
	assert.Empty(t, tree.Findings)
	assert.Zero(t, calls)
}
