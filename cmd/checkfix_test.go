package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single command checks and repairs everything. Without --fix it
// reports and writes nothing; with --fix it repairs what it can and reports the rest.
func TestCheckFixRepairsInPlace(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	body := "# Title\n\nIt doesn't hold the lock.\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	checkFix = false
	_ = runCheck(rootCmd, []string{path})
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, body, string(after), "a check writes nothing")

	checkFix = true
	t.Cleanup(func() { checkFix = false })
	require.NoError(t, runCheck(rootCmd, []string{path}))

	fixed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(fixed), "doesn't")
	assert.Contains(t, string(fixed), "# Title")
}

// A comment finding on a named file fails the check. The report carries it as a
// kept hit, and a check that read only the other list passed an essay.
func TestCheckReportsAKeptHitOnANamedFile(t *testing.T) {
	t.Serial()
	path := filepath.Join(t.TempDir(), "a.go")
	body := "package p\n\n" +
		"// FullMipChain marks a TextureBinding as carrying its complete mip pyramid\n" +
		"// when the exact level count is not supplied (--tex ...:mips).\n" +
		"const FullMipChain = -1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	checkFix = false
	assert.ErrorIs(t, runCheck(rootCmd, []string{path}), errFindings)
}

// A build names a directory rather than every file under it, so a directory is
// the tree beneath it. Reading the directory as a file is what a build sees as
// "read .: is a directory".
func TestCheckTakesADirectoryAsTheTreeUnderIt(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	nested := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	path := filepath.Join(nested, "README.md")
	body := "# Title\n\nIt doesn't hold the lock.\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	checkFix = false
	_ = runCheck(rootCmd, []string{dir})
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, body, string(after), "a check over a tree writes nothing")

	checkFix = true
	t.Cleanup(func() { checkFix = false })
	require.NoError(t, runCheck(rootCmd, []string{dir}))

	fixed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(fixed), "doesn't", "the file under the directory was not repaired")
}
