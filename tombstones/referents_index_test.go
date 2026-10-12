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

// A name that a listed path spells names that file. A trailing underscore names
// every file that starts with it.
func TestANameThatSpellsAFileIsAlive(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", root, "init", "-q").Run())
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "defs_darwin_amd64.c"), []byte("int x;\n"), 0o644))
	path := filepath.Join(root, "b.c")
	PrimeIndex(path)

	text := "// see defs_darwin_amd64, every defs_darwin_ file and gone_file_name"
	blocks := []Block{{Text: text, Prose: text, LineNos: []int{0}}}
	assert.Equal(t, []string{"gone_file_name"}, DeadReferents(path, "", blocks))
}

// A Go comment that names a C function or a kernel field is not a tombstone.
func TestAnUnderscoredNameInGoIsForeign(t *testing.T) {
	assert.True(t, foreignInGo("a.go", "epoll_pwait2"))
	assert.True(t, foreignInGo("a.go", "x86_thread_state64"))
	assert.False(t, foreignInGo("a.go", "Test_parseFlags"))
	assert.False(t, foreignInGo("a.go", "parseLegacyFlag"))
	assert.False(t, foreignInGo("a.c", "epoll_pwait2"))
}

func TestAddWordsKeepsOnlyCandidates(t *testing.T) {
	names := set.New[string]()
	addWords(names, []byte("x = CamelCaseName + lowercase_word + SHOUTING + short\n"))
	assert.True(t, names.Contains("CamelCaseName"))
	assert.True(t, names.Contains("lowercase_word"))
	assert.False(t, names.Contains("SHOUTING"), "all capitals is never a candidate")
	assert.False(t, names.Contains("short"))
}
