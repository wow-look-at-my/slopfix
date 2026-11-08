// foundgone_test.go holds the tests for a recorded repository that the session deleted.

package noworkloss

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestADeletedRepositoryDoesNotBlockTheStop(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	session := "deleted-worktree-session"
	repo := filepath.Join(t.TempDir(), "scratch-worktree")
	out, err := exec.Command("git", "init", "-q", repo).CombinedOutput()
	require.Nil(t, err, "git init: %s", out)
	got := recordRepos(session, []string{repo})
	require.Equal(t, 1, len(got), "the repository must be recorded: %v", got)
	require.NoError(t, os.RemoveAll(repo))

	res := foundAtStop(hookInput{HookEventName: "Stop", SessionID: session})
	require.NotContains(t, res.Stdout, `"block"`, "a deleted repository holds no work, so the stop must pass: %s", res.Stdout)
}
