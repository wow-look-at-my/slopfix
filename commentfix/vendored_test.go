package commentfix

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A path .gitattributes marks linguist-vendored has another author. The walk
// leaves it out, the way it leaves out vendor/ and a submodule.
func TestTheWalkSkipsAPathMarkedVendored(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "isa"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("isa/** linguist-vendored\nmanual.txt linguist-vendored\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "isa", "ch01.go"), []byte("package p\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "manual.go"), []byte("package p\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "own.go"), []byte("package p\n"), 0o644))

	paths := []string{filepath.Join(root, "isa", "ch01.go"), filepath.Join(root, "manual.go"), filepath.Join(root, "own.go")}
	kept := withoutVendored(root, paths)
	assert.Equal(t, []string{filepath.Join(root, "manual.go"), filepath.Join(root, "own.go")}, kept)

	files := TreeFiles(root)
	assert.Contains(t, files, filepath.Join(root, "own.go"))
	assert.NotContains(t, files, filepath.Join(root, "isa", "ch01.go"))
}

// A path .gitattributes marks linguist-generated is a generator's output. The
// next run writes it again, so the walk leaves it out too.
func TestTheWalkSkipsAPathMarkedGenerated(t *testing.T) {
	root := t.TempDir()
	out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput()
	require.NoError(t, err, string(out))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("docs/routes.txt linguist-generated\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "routes.txt"), []byte("/ {GET}\n/llms.txt {GET}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "own.txt"), []byte("A note.\n"), 0o644))

	files := TreeFilesMatching(root, func(string) bool { return true })
	assert.Contains(t, files, filepath.Join(root, "docs", "own.txt"))
	assert.NotContains(t, files, filepath.Join(root, "docs", "routes.txt"))
}
