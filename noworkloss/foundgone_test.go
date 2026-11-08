// foundgone_test.go holds the tests for a recorded repository that the session deleted.

package noworkloss

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADeletedRepositoryDoesNotBlockTheStop(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	session := "deleted-worktree-session"
	repo := filepath.Join(t.TempDir(), "scratch-worktree")
	out, err := exec.Command("git", "init", "-q", repo).CombinedOutput()
	require.NoError(t, err, string(out))

	got := recordRepos(session, []string{repo})
	require.Equal(t, 1, len(got))

	require.NoError(t, os.RemoveAll(repo))

	res := foundAtStop(hookInput{HookEventName: "Stop", SessionID: session})
	require.NotContains(t, res.Stdout, `"block"`)

}

// A scratch repository deleted after a run has no work left to save. The
// found-work pass skips it and does not report a git status failure.
func TestFoundWorkSkipsARootThatIsGone(t *testing.T) {
	gone := filepath.Join(newRepo(t), "deleted")
	assert.Empty(t, commitFound(gone, "s", 0))
}

func TestFoundWorkSkipsARootWithNoGitEntry(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git")))
	assert.Empty(t, commitFound(dir, "s", 0))
}

// A root that still holds a .git entry, and that git cannot read, is still a failure.
func TestFoundWorkReportsABrokenRepository(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git")))
	writeAt(t, dir, ".git", "gitdir: "+filepath.Join(dir, "missing")+"\n")
	got := commitFound(dir, "s", 0)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].failure, "git status failed")
}
