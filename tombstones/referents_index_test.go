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

// The index reads the tracked files only. An untracked file and a nested
// checkout stay out, as they do in the walk.
func TestTheIndexHoldsTheTrackedIdentifiers(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nfunc TrackedSymbolName() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package p\n\nfunc UntrackedSymbolName() {}\n"), 0o644))
	run("add", "a.go")

	ix := &symbolIndex{}
	ix.build(root)
	require.True(t, ix.ok)
	assert.True(t, ix.holds("TrackedSymbolName"))
	assert.False(t, ix.holds("UntrackedSymbolName"))
	assert.False(t, ix.holds("NothingDefinesThisName"))
}

func TestAddWordsKeepsOnlyCandidates(t *testing.T) {
	names := set.New[string]()
	addWords(names, []byte("x = CamelCaseName + lowercase_word + SHOUTING + short\n"))
	assert.True(t, names.Contains("CamelCaseName"))
	assert.True(t, names.Contains("lowercase_word"))
	assert.False(t, names.Contains("SHOUTING"), "all capitals is never a candidate")
	assert.False(t, names.Contains("short"))
}
