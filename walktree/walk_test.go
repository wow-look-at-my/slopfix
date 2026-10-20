package walktree

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// isolate gives a test its own HOME and cache directory, persists every walk,
// and starts and ends with no walk in the memo.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	floor := PersistFloor
	PersistFloor = 0
	Forget()
	t.Cleanup(func() {
		PersistFloor = floor
		Forget()
	})
	repo := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(repo, 0o755))
	return repo
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// claudeMdFiles is a fresh process's answer: no walk in the memo, the cache
// file as the last process left it.
func claudeMdFiles(root string) []string {
	Forget()
	files := Get(root).Named("CLAUDE.md")
	sort.Strings(files)
	return files
}

// cacheTree builds a tree of CLAUDE.md files whose directories are an hour
// old, so the cache trusts every listing.
func cacheTree(t *testing.T) string {
	t.Helper()
	repo := isolate(t)
	for _, p := range []string{
		"CLAUDE.md",
		"a/CLAUDE.md",
		"a/b/c/CLAUDE.md",
		"d/e/notes.md",
		"node_modules/x/CLAUDE.md",
		".git/CLAUDE.md",
	} {
		writeFile(t, filepath.Join(repo, p), "x")
	}
	ageDirs(t, repo)
	return repo
}

func ageDirs(t *testing.T, root string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		return os.Chtimes(path, old, old)
	}))
}

// uncached is the walk with no cache to start from.
func uncached(t *testing.T, root string) []string {
	t.Helper()
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	path := cachePath(abs)
	require.NotEmpty(t, path)
	require.NoError(t, os.RemoveAll(path))
	return claudeMdFiles(root)
}

func cachedListings(t *testing.T, root string) map[string]*listing {
	t.Helper()
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	return loadCache(abs)
}

func TestCachedWalkEqualsUncachedWalk(t *testing.T) {
	repo := cacheTree(t)
	want := uncached(t, repo)
	require.Equal(t, []string{
		filepath.Join(repo, "CLAUDE.md"),
		filepath.Join(repo, "a", "CLAUDE.md"),
		filepath.Join(repo, "a", "b", "c", "CLAUDE.md"),
	}, want)

	require.Equal(t, want, claudeMdFiles(repo))
	cache := cachedListings(t, repo)
	require.Len(t, cache, 6, "the root, a, a/b, a/b/c, d, d/e")
	for key, entry := range cache {
		require.NotEqual(t, int64(-1), entry.Mtime, "an old directory is trusted: %q", key)
	}
}

func TestCachedWalkFindsANewFileDeepInTheTree(t *testing.T) {
	repo := cacheTree(t)
	claudeMdFiles(repo)
	added := filepath.Join(repo, "d", "e", "CLAUDE.md")
	writeFile(t, added, "x")

	require.Contains(t, claudeMdFiles(repo), added)
	require.Equal(t, uncached(t, repo), claudeMdFiles(repo))
}

func TestCachedWalkDropsADeletedFile(t *testing.T) {
	repo := cacheTree(t)
	claudeMdFiles(repo)
	gone := filepath.Join(repo, "a", "b", "c", "CLAUDE.md")
	require.NoError(t, os.Remove(gone))

	require.NotContains(t, claudeMdFiles(repo), gone)
	require.Equal(t, uncached(t, repo), claudeMdFiles(repo))
}

func TestCachedWalkFindsANewDirectory(t *testing.T) {
	repo := cacheTree(t)
	claudeMdFiles(repo)
	added := filepath.Join(repo, "a", "b", "new", "deeper", "CLAUDE.md")
	writeFile(t, added, "x")

	require.Contains(t, claudeMdFiles(repo), added)
	require.Equal(t, uncached(t, repo), claudeMdFiles(repo))
}

func TestWalkSkipsALinkedWorktree(t *testing.T) {
	repo := cacheTree(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature")
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: "+filepath.Join(repo, ".git", "worktrees", "feature")+"\n")
	writeFile(t, filepath.Join(wt, "CLAUDE.md"), "x")
	writeFile(t, filepath.Join(wt, "sub", "CLAUDE.md"), "x")

	got := uncached(t, repo)
	require.NotContains(t, got, filepath.Join(wt, "CLAUDE.md"))
	require.NotContains(t, got, filepath.Join(wt, "sub", "CLAUDE.md"))
	require.Equal(t, got, claudeMdFiles(repo))
}

func TestWalkEntersASubmodule(t *testing.T) {
	repo := cacheTree(t)
	sub := filepath.Join(repo, "vendor", "lib")
	writeFile(t, filepath.Join(sub, ".git"), "gitdir: ../../.git/modules/vendor/lib\n")
	writeFile(t, filepath.Join(sub, "CLAUDE.md"), "x")

	require.Contains(t, uncached(t, repo), filepath.Join(sub, "CLAUDE.md"))
}

func TestWalkEntersARootThatIsAWorktree(t *testing.T) {
	repo := isolate(t)
	writeFile(t, filepath.Join(repo, ".git"), "gitdir: /elsewhere/.git/worktrees/repo\n")
	writeFile(t, filepath.Join(repo, "a", "CLAUDE.md"), "x")

	require.Contains(t, uncached(t, repo), filepath.Join(repo, "a", "CLAUDE.md"))
}

func TestCorruptWalkCacheIsAFullWalk(t *testing.T) {
	repo := cacheTree(t)
	want := uncached(t, repo)
	abs, err := filepath.Abs(repo)
	require.NoError(t, err)
	writeFile(t, cachePath(abs), "not a gob")

	require.Equal(t, want, claudeMdFiles(repo))
	require.Len(t, cachedListings(t, repo), 6, "the walk rewrote the cache")
}

// A second Get for a root, or for a directory inside it, reads nothing.
func TestASecondGetReadsNothing(t *testing.T) {
	repo := cacheTree(t)
	first := Get(repo).Files()
	require.NotEmpty(t, first)

	Ops.Store(0)
	require.Equal(t, first, Get(repo).Files())
	require.Equal(t, []string{filepath.Join(repo, "a", "b", "c", "CLAUDE.md")}, Get(filepath.Join(repo, "a", "b")).Files())
	require.Zero(t, Ops.Load(), "a walked tree answers without a read or a stat")
}

func TestCreatedAndRemovedKeepTheMemoTrue(t *testing.T) {
	repo := cacheTree(t)
	Get(repo)
	added := filepath.Join(repo, "x", "y", "CLAUDE.md")
	writeFile(t, added, "x")
	Created(added)
	require.Contains(t, Get(repo).Named("CLAUDE.md"), added)

	gone := filepath.Join(repo, "a", "CLAUDE.md")
	require.NoError(t, os.Remove(gone))
	Removed(gone)
	require.NotContains(t, Get(repo).Named("CLAUDE.md"), gone)

	require.NoError(t, os.RemoveAll(filepath.Join(repo, "a", "b")))
	Removed(filepath.Join(repo, "a", "b"))
	require.NotContains(t, Get(repo).Named("CLAUDE.md"), filepath.Join(repo, "a", "b", "c", "CLAUDE.md"))
}

func TestVisitSkipsADirectoryEnterRefuses(t *testing.T) {
	repo := cacheTree(t)
	var files []string
	Get(repo).Visit(func(dir string) bool { return filepath.Base(dir) != "b" }, func(path string, _ Entry) {
		files = append(files, path)
	})
	require.Contains(t, files, filepath.Join(repo, "a", "CLAUDE.md"))
	require.NotContains(t, files, filepath.Join(repo, "a", "b", "c", "CLAUDE.md"))
}
