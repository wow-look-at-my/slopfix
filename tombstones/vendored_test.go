package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A file .gitattributes marks linguist-vendored is borrowed text. A named
// path and the hook both leave it as it is.
func TestAPathMarkedVendoredIsBorrowed(t *testing.T) {
	root := t.TempDir()
	out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput()
	require.NoError(t, err, string(out))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "isa"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("isa/** linguist-vendored\n"), 0o644))
	manual := filepath.Join(root, "isa", "ch01.txt")
	own := filepath.Join(root, "notes.txt")
	doc := "This used to be the old way.\n"
	require.NoError(t, os.WriteFile(manual, []byte(doc), 0o644))
	require.NoError(t, os.WriteFile(own, []byte(doc), 0o644))

	assert.True(t, Borrowed(manual))
	assert.False(t, Borrowed(own))

	repair := Fix(manual, doc, DefaultMaxCommentLines)
	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Kept)
}

// A file .gitattributes marks linguist-generated is a generator's output, and
// is borrowed the same way.
func TestAPathMarkedGeneratedIsBorrowed(t *testing.T) {
	root := t.TempDir()
	out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput()
	require.NoError(t, err, string(out))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("routes.txt linguist-generated\nown.txt -linguist-generated\n"), 0o644))
	routes := filepath.Join(root, "routes.txt")
	own := filepath.Join(root, "own.txt")
	require.NoError(t, os.WriteFile(routes, []byte("/ {GET}\n"), 0o644))
	require.NoError(t, os.WriteFile(own, []byte("A note.\n"), 0o644))

	assert.True(t, Borrowed(routes))
	assert.False(t, Borrowed(own))
}
