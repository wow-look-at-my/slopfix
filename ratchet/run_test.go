package ratchet

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeIn writes content to name under dir, making its directory.
func writeIn(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

// gitIn runs git in dir.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Run(), "git %v failed", args)
}

// newClone builds an origin whose master holds files, and answers a clone of
// it on a feature branch.
func newClone(t *testing.T, files map[string]string) string {
	t.Helper()
	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "--bare", "-b", "master")

	seed := t.TempDir()
	gitIn(t, seed, "init", "-q", "-b", "master")
	gitIn(t, seed, "config", "user.email", "t@example.com")
	gitIn(t, seed, "config", "user.name", "t")
	for name, content := range files {
		writeIn(t, seed, name, content)
	}
	gitIn(t, seed, "add", "-A")
	gitIn(t, seed, "commit", "-qm", "init")
	gitIn(t, seed, "push", "-q", "file://"+origin, "master")

	clone := t.TempDir()
	gitIn(t, clone, "clone", "-q", "file://"+origin, ".")
	gitIn(t, clone, "config", "user.email", "t@example.com")
	gitIn(t, clone, "config", "user.name", "t")
	gitIn(t, clone, "checkout", "-q", "-b", "feature")
	return clone
}

// commitIn writes files in dir and commits them.
func commitIn(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		writeIn(t, dir, name, content)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "change")
}

// inCIOn sets the environment of a CI run for branch.
func inCIOn(t *testing.T, branch string) {
	t.Helper()
	t.Setenv("CI", "true")
	t.Setenv("GITHUB_REF_NAME", branch)
	t.Setenv(byParentVar, "")
}

// judged is a repository whose master holds a guarantee: code.go says strong,
// and master's judge.sh fails a branch whose code.go does not.
var judged = map[string]string{
	File:       "# master judges every branch\nsh judge.sh\n",
	"judge.sh": "grep -q strong \"$1/code.go\" || { echo \"weakened: $1/code.go\"; exit 1; }\n",
	"code.go":  "package x // strong\n",
}

// A branch that keeps the guarantee passes, and may change anything else.
// With go-toolchain on PATH, go names it, and the restore puts PATH back.
func TestOrgGoRunsGoToolchainAsGo(t *testing.T) {
	t.Serial()
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go-toolchain"), []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", bin)
	restore, err := orgGo()
	require.NoError(t, err)
	goPath, err := exec.LookPath("go")
	require.NoError(t, err)
	target, err := os.Readlink(goPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(bin, "go-toolchain"), target)
	restore()
	assert.Equal(t, bin, os.Getenv("PATH"))
}

func TestABranchThatKeepsTheGuaranteePasses(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{"code.go": "package x // strong, and better\n", "other.go": "package x\n"})

	assert.NoError(t, CheckIn(clone))
}

// A branch that weakens what master judges fails.
func TestABranchThatWeakensTheGuaranteeFails(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{"code.go": "package x // weak\n"})

	err := CheckIn(clone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master's .github/ratchet (sh judge.sh) fails on this branch")
}

// The judge is master's copy. A branch that rewrites it to pass, and then
// weakens the code, still fails.
func TestABranchCannotRewriteItsJudge(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{"judge.sh": "exit 0\n", "code.go": "package x // weak\n"})

	assert.Error(t, CheckIn(clone))
}

// The command is master's too. A branch that points the file at a command that
// always passes still fails.
func TestABranchCannotRenameItsJudge(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{File: "true\n", "code.go": "package x // weak\n"})

	assert.Error(t, CheckIn(clone))
}

// The judge runs from a checkout of master, and the run leaves no worktree
// behind.
func TestTheJudgeRunsFromMastersCheckout(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	files := map[string]string{
		File:             "sh judge.sh\n",
		"judge.sh":       "test -f only-on-master && test \"$(pwd)\" != \"$1\"\n",
		"only-on-master": "",
	}
	clone := newClone(t, files)
	gitIn(t, clone, "rm", "-q", "only-on-master")
	gitIn(t, clone, "commit", "-qm", "drop")

	require.NoError(t, CheckIn(clone))
	worktrees, err := os.ReadDir(filepath.Join(clone, ".git", "worktrees"))
	if err == nil {
		assert.Empty(t, worktrees, "the run leaves no worktree behind")
	}
}

// The default branch is where an owner's merge lands a change to the judge, so
// its own run passes.
func TestTheDefaultBranchItselfPasses(t *testing.T) {
	t.Serial()
	inCIOn(t, "master")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{"code.go": "package x // weak\n"})

	assert.NoError(t, CheckIn(clone))
}

// A repository whose default branch names no ratchet has nothing to hold a
// branch to. A file the branch adds counts once it lands on master.
func TestNoRatchetJudgesNothing(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, map[string]string{"code.go": "package x // strong\n"})
	commitIn(t, clone, map[string]string{File: "false\n"})

	assert.NoError(t, CheckIn(clone))
}

// A ratchet file with no command is an error, never a pass.
func TestAnEmptyRatchetFails(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, map[string]string{File: "# nothing\n\n"})

	err := CheckIn(clone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "names no command")
}

// Outside CI the check never reaches the network.
func TestRatchetSkipsOutsideCI(t *testing.T) {
	t.Serial()
	t.Setenv("CI", "")
	assert.NoError(t, CheckIn(t.TempDir()))
	assert.NoError(t, Check())
}

// A child run under the build it judges leaves the ratchet to its parent.
func TestAChildLeavesTheRatchetToItsParent(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	clone := newClone(t, judged)
	commitIn(t, clone, map[string]string{"code.go": "package x // weak\n"})

	t.Setenv(byParentVar, "1")
	assert.NoError(t, CheckIn(clone))
	t.Setenv(byParentVar, "")
	assert.Error(t, CheckIn(clone))
}

// A directory that is no repository has nothing to compare, so the run goes
// on to say what it does lack.
func TestADirectoryOutsideARepositoryHasNoRatchet(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	assert.NoError(t, CheckIn(t.TempDir()))
}

// A CI run that cannot learn the default branch fails rather than passing
// unchecked.
func TestAnUnreachableOriginFails(t *testing.T) {
	t.Serial()
	inCIOn(t, "feature")
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "master")

	err := CheckIn(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading the default branch")
}

func TestCommandSkipsCommentsAndBlanks(t *testing.T) {
	t.Serial()
	assert.Equal(t, []string{"go", "run", "./cmd/ratchet"}, Command("# note\n\n  go run ./cmd/ratchet  \nignored\n"))
	assert.Empty(t, Command("# only a comment\n\n"))
}

// A checkout with no go.mod generates nothing.
func TestGenerateInSkipsADirectoryWithoutAGoMod(t *testing.T) {
	t.Serial()
	assert.NoError(t, generateIn(t.TempDir()))
}
