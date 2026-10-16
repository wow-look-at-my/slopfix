package forkscope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The work tree is diffed against the base in-process: a changed file holds
// its new lines, a new file and a binary are whole. A deleted file holds
// nothing. A line that reads like a diff header is plain text here.
func TestTheWorkTreeDiffCountsWhatTheForkWrote(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	writeT(t, dir, "x.md", "one\ntwo\nthree\n")
	writeT(t, dir, "gone.md", "gone\n")
	writeT(t, dir, "data.bin", "\x00base")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "base")
	base := gitT(t, dir, "rev-parse", "HEAD")

	writeT(t, dir, "x.md", "one\n+++ b/fake.md\n@@ -1 +1,9 @@\nthree\n")
	writeT(t, dir, "sp ace\t.md", "new\n")
	writeT(t, dir, "data.bin", "\x00fork")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.md")))
	gitT(t, dir, "add", "-A")

	own, err := (&Base{top: dir, commit: base}).Lines()
	require.NoError(t, err)
	at := func(name string) string { return filepath.Join(dir, name) }
	assert.True(t, own.Holds(at("x.md"), 2, 3))
	assert.False(t, own.Holds(at("x.md"), 1, 1))
	assert.False(t, own.Holds(at("x.md"), 4, 4))
	assert.True(t, own.Whole(at("sp ace\t.md")))
	assert.True(t, own.Whole(at("data.bin")), "a binary the fork changed is whole")
	assert.False(t, own.Holds(at("gone.md"), 1, 1))
	assert.False(t, own.Holds(at("untouched.md"), 0, 0))
}

// The staged selector diffs the index against HEAD, and a rev selector the
// work tree against that revision.
func TestDiffLinesReadsTheIndexAndARevision(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	writeT(t, dir, "a.md", "one\ntwo\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "base")
	writeT(t, dir, "a.md", "one\ntwo\nstaged\n")
	gitT(t, dir, "add", "a.md")
	writeT(t, dir, "a.md", "one\ntwo\nstaged\nunstaged\n")

	staged, err := DiffLines(dir, "", true)
	require.NoError(t, err)
	assert.True(t, staged.Holds(filepath.Join(dir, "a.md"), 3, 3))
	assert.False(t, staged.Holds(filepath.Join(dir, "a.md"), 4, 4))

	worked, err := DiffLines(dir, "HEAD", false)
	require.NoError(t, err)
	assert.True(t, worked.Holds(filepath.Join(dir, "a.md"), 3, 4))
	assert.False(t, worked.Holds(filepath.Join(dir, "a.md"), 1, 2))

	_, err = DiffLines(dir, "HEAD@{0}", false)
	assert.NoError(t, err, "git parses a reflog spelling")
}
