package forkscope

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poisonGit puts a git on PATH that records every spawn in a marker file, then
// exits nonzero. It answers the marker path.
func poisonGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "spawned")
	script := "#!/bin/sh\nprintf spawned >> \"$SLOPFIX_GIT_MARKER\"\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("SLOPFIX_GIT_MARKER", marker)
	t.Setenv("PATH", dir)
	return marker
}

// spawnedInGit reports whether anything ran the git on PATH.
func spawnedInGit(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// The fork scope reads the repository itself, so the poisoned git is never run.
func TestTheForkScopeStartsNoGitProcess(t *testing.T) {
	fx := newForkFixture(t)
	env := forkEnv(forkAPI(t, forkBody(fx.parent)))
	marker := poisonGit(t)

	own, err := forkLines(fx.fork, env, noList(t))
	require.NoError(t, err)
	require.NotNil(t, own)
	assert.False(t, spawnedInGit(marker), "a git process was spawned on the fork-scope path")
	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...))
	assert.Equal(t, []int{3}, held(own, fx.fork, "new.md", 3))
}

// A listed fork reads its upstream and its changed files without a git process.
func TestAListedForkStartsNoGitProcess(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	list := forkList("o/fork", fx.parent)
	marker := poisonGit(t)

	own, err := forkLines(fx.fork, listedEnv(t), listAt(t, http.StatusOK, list))
	require.NoError(t, err)
	require.NotNil(t, own)
	assert.False(t, spawnedInGit(marker), "a git process was spawned on the listed-fork path")
	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...))
	assert.Equal(t, []int{3}, held(own, fx.fork, "loose.md", 3))
}
