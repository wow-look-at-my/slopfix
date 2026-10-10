package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// The index reads what ripgrep reads: tracked files, and untracked files git
// does not ignore. An ignored file stays out.
func TestTheIndexHoldsWhatAProbeReads(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nfunc TrackedSymbolName() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package p\n\nfunc UntrackedSymbolName() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("out.go\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "out.go"), []byte("package p\n\nfunc IgnoredSymbolName() {}\n"), 0o644))
	run("add", "a.go")

	ix := &symbolIndex{}
	ix.build(root)
	require.True(t, ix.ok)
	assert.True(t, ix.holds("TrackedSymbolName"))
	assert.True(t, ix.holds("UntrackedSymbolName"))
	assert.False(t, ix.holds("IgnoredSymbolName"))
	assert.False(t, ix.holds("NothingDefinesThisName"))
}

// A checked-out submodule is part of the tree, so the names it defines are alive.
func TestTheIndexReadsASubmodule(t *testing.T) {
	base := t.TempDir()
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "protocol.file.allow=always", "-c", "user.name=t", "-c", "user.email=t@e"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	sub := filepath.Join(base, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	run(sub, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(sub, "s.go"), []byte("package s\n\nfunc SubmoduleSymbolName() {}\n"), 0o644))
	run(sub, "add", "s.go")
	run(sub, "commit", "-q", "-m", "s")
	root := filepath.Join(base, "root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	run(root, "init", "-q")
	run(root, "submodule", "add", "-q", sub, "dep")

	ix := &symbolIndex{}
	ix.build(root)
	require.True(t, ix.ok)
	assert.True(t, ix.holds("SubmoduleSymbolName"))
}

// Prose writes a name as a plural, or with a capital at a sentence start, and
// both forms answer to the name. A star or a trailing underscore is a pattern.
func TestNameFormsAndPatterns(t *testing.T) {
	assert.Equal(t, []string{"OpConstants", "OpConstant", "opConstants"}, nameForms("OpConstants"))
	assert.Equal(t, []string{"Vid_convert_roundtrip", "vid_convert_roundtrip"}, nameForms("Vid_convert_roundtrip"))
	assert.NotContains(t, ownNames("the DotProductAccelerated* features"), "DotProductAccelerated")
	assert.NotContains(t, ownNames("every VK_KHR_pipeline_executable_ entry point"), "VK_KHR_pipeline_executable_")
}

// A name that is the stem of a file here names that file, so it is alive. A
// file name that no file answers is still dead.
func TestAFileStemIsAlive(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", root, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(root, "xattr_windows.go"), []byte("package p\n"), 0o644))
	src := "package p\n\n// Semantics mirror xattr_windows.go, not xattr_plan9.go.\nfunc f() {}\n"
	path := filepath.Join(root, "x.go")
	assert.Equal(t, []string{"xattr_plan9"}, DeadReferents(path, src, AddedBlocks(path, src)))
}

// Once a walk primes the index, it answers every name without a probe: a name
// it never saw is dead. A name it holds is alive.
func TestAPrimedIndexAnswersWithoutAProbe(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init", "-q")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nfunc LivingSymbolName() {}\n"), 0o644))
	path := filepath.Join(root, "b.go")
	PrimeIndex(path)
	t.Setenv("PATH", "")

	text := "// LivingSymbolName and GoneSymbolName"
	blocks := []Block{{Text: text, Prose: text, LineNos: []int{0}}}
	assert.Equal(t, []string{"GoneSymbolName"}, DeadReferents(path, "", blocks))
}

func TestAddWordsKeepsOnlyCandidates(t *testing.T) {
	names := set.New[string]()
	addWords(names, []byte("x = CamelCaseName + lowercase_word + SHOUTING + short\n"))
	assert.True(t, names.Contains("CamelCaseName"))
	assert.True(t, names.Contains("lowercase_word"))
	assert.False(t, names.Contains("SHOUTING"), "all capitals is never a candidate")
	assert.False(t, names.Contains("short"))
}
