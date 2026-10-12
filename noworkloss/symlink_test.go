package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// No edit tool can make a symlink. A link to a file in the same tree adds no
// text, so it runs.
func TestASymlinkToAFileInTheTreeRuns(t *testing.T) {
	dir := newRepo(t)
	writeAt(t, dir, "sub/copy.go", "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "copy")

	allowed(t, dir, "ln -s tracked.go link.go")
	allowed(t, dir, "ln -sf ../tracked.go sub/copy.go")
	allowed(t, dir, "ln --symbolic --force ../tracked.go sub/copy.go")
	allowed(t, dir, "cd sub && ln -sf ../tracked.go copy.go")
	allowed(t, dir, "ln -s ../tracked.go sub")
}

// A link that leads out of the tree brings in text no edit tool wrote.
func TestASymlinkOutOfTheTreeIsRefused(t *testing.T) {
	dir := newRepo(t)
	out := t.TempDir()
	writeAt(t, out, "staged.go", "package a\n")
	rel, err := filepath.Rel(dir, filepath.Join(out, "staged.go"))
	require.NoError(t, err)

	assert.Contains(t, denied(t, dir, "ln -s "+filepath.Join(out, "staged.go")+" link.go"), "link.go")
	assert.Contains(t, denied(t, dir, "ln -s "+rel+" link.go"), "link.go")
	assert.Contains(t, denied(t, dir, "ln -s build/gen.go link.go"), "link.go")
	assert.Contains(t, denied(t, dir, "ln tracked.go link.go"), "link.go")
}

// A target that is itself a link must also end inside the tree.
func TestASymlinkThroughALinkThatLeavesTheTreeIsRefused(t *testing.T) {
	dir := newRepo(t)
	out := t.TempDir()
	writeAt(t, out, "staged.go", "package a\n")
	require.NoError(t, os.Symlink(filepath.Join(out, "staged.go"), filepath.Join(dir, "escape.go")))

	assert.Contains(t, denied(t, dir, "ln -s escape.go link.go"), "link.go")
}

// A forced link replaces the file at its path, so an edit there is kept first.
func TestAForcedSymlinkOverAnEditKeepsTheEdit(t *testing.T) {
	dir := newRepo(t)
	writeAt(t, dir, "sub/copy.go", "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "copy")
	writeAt(t, dir, "sub/copy.go", "package a\n// edited\n")

	reason, notices := lossOnlyNotices(t, dir, "ln -sf ../tracked.go sub/copy.go")
	assert.Empty(t, reason)
	require.NotEmpty(t, notices)
	assert.Contains(t, notices[0], "1 modified")
}
