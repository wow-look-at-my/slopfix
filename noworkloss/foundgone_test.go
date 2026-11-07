package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A scratch repository deleted after a run has no work left to save. The
// found-work pass skips it and does not report a git status failure.
func TestFoundWorkSkipsARootThatIsGone(t *testing.T) {
	gone := filepath.Join(newRepo(t), "deleted")
	assert.Empty(t, commitFound(gone, "s", 0))
}

func TestFoundWorkSkipsARootWithNoGitEntry(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git")))
	assert.Empty(t, commitFound(dir, "s", 0))
}

// A root that still holds a .git entry, and that git cannot read, is still a failure.
func TestFoundWorkReportsABrokenRepository(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git")))
	writeAt(t, dir, ".git", "gitdir: "+filepath.Join(dir, "missing")+"\n")
	got := commitFound(dir, "s", 0)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].failure, "git status failed")
}
