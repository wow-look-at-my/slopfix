// foundgone_test.go holds the tests for a recorded repository that the session deleted.

package noworkloss

import (
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
