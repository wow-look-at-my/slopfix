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
// it never saw is dead, and a name it holds is alive.
func TestAPrimedIndexAnswersWithoutAProbe(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init", "-q")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nfunc LivingSymbolName() {}\n"), 0o644))
	path := filepath.Join(root, "b.go")
	PrimeIndex(path)
	t.Setenv("PATH", "")

	blocks := []Block{{Text: "// LivingSymbolName and GoneSymbolName", LineNos: []int{0}}}
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
