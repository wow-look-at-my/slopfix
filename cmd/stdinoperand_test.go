package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forbiddenStdin fails the test that reads it.
type forbiddenStdin struct{ t *testing.T }

func (f forbiddenStdin) Read([]byte) (int, error) {
	f.t.Helper()
	f.t.Error("stdin was read though no operand named it")
	return 0, assert.AnError
}

// commandWith builds a command whose stdin is in and whose stdout is the buffer returned.
func commandWith(in interface{ Read([]byte) (int, error) }) (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(in)
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	return cmd, &out
}

// pathFlag sets --path for one test and restores it.
func pathFlag(t *testing.T, path string) {
	t.Helper()
	old := checkPath
	checkPath = path
	t.Cleanup(func() { checkPath = old })
}

// With no operand, check walks the current directory and never touches stdin.
func TestBareCheckWalksTheCurrentDirectory(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	put(t, repo, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, false)

	cmd, out := commandWith(forbiddenStdin{t})
	assert.ErrorIs(t, runCheck(cmd, nil), errFindings)
	assert.Contains(t, out.String(), "README.md")
	assert.Equal(t, guardBody, readT(t, repo+"/README.md"), "a check writes nothing")
}

// A clean tree passes with no operand and an empty stdin it must not wait on.
func TestBareCheckOfACleanTreePasses(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	put(t, repo, "README.md", "# Title\n\nIt does not hold the lock.\n")
	t.Chdir(repo)
	fixMode(t, false)

	cmd, _ := commandWith(forbiddenStdin{t})
	require.NoError(t, runCheck(cmd, nil))
}

// Bare fix repairs the tree under the current directory.
func TestBareFixRepairsTheCurrentDirectory(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	put(t, repo, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, true)

	cmd, _ := commandWith(forbiddenStdin{t})
	require.NoError(t, runCheck(cmd, nil))
	assert.NotContains(t, readT(t, repo+"/README.md"), "doesn't")
}

// The operand - names stdin, and --path says which file the text is headed for.
func TestDashReadsStdin(t *testing.T) {
	t.Serial()
	fixMode(t, false)
	pathFlag(t, "README.md")

	cmd, _ := commandWith(strings.NewReader(guardBody))
	assert.ErrorIs(t, runCheck(cmd, []string{"-"}), errFindings)
}

// fix on - writes the repaired text on stdout and needs no work tree for it.
func TestDashFixWritesTheRepairToStdout(t *testing.T) {
	t.Serial()
	t.Chdir(t.TempDir())
	fixMode(t, true)
	pathFlag(t, "README.md")

	cmd, out := commandWith(strings.NewReader(guardBody))
	require.NoError(t, runCheck(cmd, []string{"-"}))
	assert.NotContains(t, out.String(), "doesn't")
	assert.Contains(t, out.String(), "# Title")
}

// --path is the file stdin is headed for, so it means nothing without -.
func TestPathWithoutDashIsAnError(t *testing.T) {
	t.Serial()
	fixMode(t, false)
	pathFlag(t, "README.md")

	cmd, _ := commandWith(forbiddenStdin{t})
	err := runCheck(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"-"`)
}

// Stdin is one file, so it cannot share a run with a path.
func TestDashBesideAPathIsAnError(t *testing.T) {
	t.Serial()
	fixMode(t, false)

	cmd, _ := commandWith(forbiddenStdin{t})
	err := runCheck(cmd, []string{"-", "README.md"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beside another path")
}

// Text on stdin is no work tree, so a line selector has nothing to select from.
func TestDashWithALineSelectorIsAnError(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	t.Chdir(repo)
	fixMode(t, false)
	old := checkStaged
	checkStaged = true
	t.Cleanup(func() { checkStaged = old })

	cmd, _ := commandWith(forbiddenStdin{t})
	err := runCheck(cmd, []string{"-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot judge stdin")
}

// A line selector over the current directory needs no operand.
func TestBareSelectorScopesTheCurrentDirectory(t *testing.T) {
	t.Serial()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	put(t, repo, "README.md", guardBody)
	t.Chdir(repo)
	fixMode(t, false)
	old := checkStaged
	checkStaged = true
	t.Cleanup(func() { checkStaged = old })

	cmd, _ := commandWith(forbiddenStdin{t})
	require.NoError(t, runCheck(cmd, nil), "nothing is staged, so nothing is selected")
}
