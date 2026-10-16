package mdbudget

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cacheTree builds a tree of CLAUDE.md files under a private HOME and cache
// directory. That cacheTree is with every directory's mtime an hour old so
// the cache trusts it.
func cacheTree(t *testing.T) string {
	t.Helper()
	repo := isolate(t)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(filepath.Dir(repo), "cache"))
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
	path := walkCachePath(root)
	require.NotEmpty(t, path)
	require.NoError(t, os.RemoveAll(path))
	return claudeMdFiles(root)
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
	cache := loadWalkCache(repo)
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

func TestCorruptWalkCacheIsAFullWalk(t *testing.T) {
	repo := cacheTree(t)
	want := uncached(t, repo)
	path := walkCachePath(repo)
	writeFile(t, path, "not a gob")

	require.Equal(t, want, claudeMdFiles(repo))
	require.Len(t, loadWalkCache(repo), 6, "the walk rewrote the cache")
}
