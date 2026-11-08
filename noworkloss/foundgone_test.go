// foundgone_test.go holds the tests for a recorded repository that the session deleted.

package noworkloss

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestADeletedRepositoryDoesNotBlockTheStop(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	session := "deleted-worktree-session"
	repo := filepath.Join(t.TempDir(), "scratch-worktree")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if got := recordRepos(session, []string{repo}); len(got) != 1 {
		t.Fatalf("the repository must be recorded, got %v", got)
	}
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}

	res := foundAtStop(hookInput{HookEventName: "Stop", SessionID: session})
	if strings.Contains(res.Stdout, `"block"`) {
		t.Fatalf("a deleted repository holds no work, so the stop must pass: %s", res.Stdout)
	}
}
