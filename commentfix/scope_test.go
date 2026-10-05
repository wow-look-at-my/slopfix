package commentfix_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
)

// run runs git in dir with no user or system config, so a signing key or a
// hook on the machine cannot change what the test sees.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// branched is a repository whose master carries old.go, with a branch on top
// that adds new.go. Both state a tally.
func branched(t *testing.T) string {
	t.Helper()
	root := tree(t, map[string]string{
		"go.mod": "module example.com/m\n",
		"old.go": "package m\n\n// It holds 3 keys.\nvar a int\n",
	})
	run(t, root, "init", "-q", "-b", "master")
	run(t, root, "add", "-A")
	run(t, root, "commit", "-q", "-m", "base")
	run(t, root, "checkout", "-q", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(root, "new.go"), []byte("package m\n\n// It holds 4 keys.\nvar b int\n"), 0o644))
	run(t, root, "add", "-A")
	run(t, root, "commit", "-q", "-m", "change")
	return root
}

func read(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	require.NoError(t, err)
	return string(b)
}

// A file the branch never touched belongs to whoever last changed it, so the
// sweep leaves it alone and rewrites only what the branch changed.
func TestTheSweepWritesOnlyWhatTheBranchChanged(t *testing.T) {
	root := branched(t)
	result := commentfix.FixTree(root)

	assert.Empty(t, result.Skipped)
	assert.Contains(t, read(t, root, "old.go"), "3 keys", "a file off the branch is not rewritten")
	assert.NotContains(t, read(t, root, "new.go"), "4 keys")
	require.Len(t, result.Rewrites, 1)
	assert.Equal(t, filepath.Join(root, "new.go"), result.Rewrites[0].Path)
	assert.Equal(t, commentfix.ID, result.Rewrites[0].Rule)
	assert.Contains(t, result.Rewrites[0].Diff, "-// It holds 4 keys.")
}

// A file not yet committed is a change of the branch too.
func TestTheSweepWritesAnUncommittedFile(t *testing.T) {
	root := branched(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "wip.go"), []byte("package m\n\n// It holds 5 keys.\nvar c int\n"), 0o644))
	commentfix.FixTree(root)
	assert.NotContains(t, read(t, root, "wip.go"), "5 keys")
}

// A rewrite in the middle of a merge or a rebase mixes into the resolution the
// user is making, so the sweep writes nothing. The sweep says why.
func TestTheSweepWritesNothingWhileGitWaitsForTheUser(t *testing.T) {
	for marker, name := range map[string]string{
		"MERGE_HEAD":       "a merge",
		"rebase-merge":     "a rebase",
		"rebase-apply":     "a rebase",
		"CHERRY_PICK_HEAD": "a cherry-pick",
	} {
		t.Run(marker, func(t *testing.T) {
			root := branched(t)
			require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", marker), 0o755))
			result := commentfix.FixTree(root)

			assert.Contains(t, result.Skipped, name)
			assert.Empty(t, result.Rewrites)
			assert.Contains(t, read(t, root, "new.go"), "4 keys", "nothing is written")
		})
	}
}

// forked is a fork of a parent that carries up.go and both.go. The fork adds
// a line to both.go. Its origin names o/fork on GitHub, and the resolver it
// answers finds the parent through an API that says so.
func forked(t *testing.T, status int) (string, forkscope.Resolver) {
	t.Helper()
	work := tree(t, map[string]string{
		"go.mod":  "module example.com/m\n",
		"up.go":   "package m\n\n// It holds 3 keys.\nvar a int\n",
		"both.go": "package m\n\n// It holds 3 keys.\nvar b int\n",
	})
	run(t, work, "init", "-q", "-b", "master")
	run(t, work, "add", "-A")
	run(t, work, "commit", "-q", "-m", "base")
	parent := filepath.Join(t.TempDir(), "parent.git")
	run(t, work, "clone", "-q", "--bare", work, parent)

	fork := filepath.Join(t.TempDir(), "fork")
	run(t, work, "clone", "-q", parent, fork)
	require.NoError(t, os.WriteFile(filepath.Join(fork, "both.go"), []byte("package m\n\n// It holds 3 keys.\nvar b int\n\n// It holds 6 locks.\nvar c int\n"), 0o644))
	run(t, fork, "add", "-A")
	run(t, fork, "commit", "-q", "-m", "fork work")
	run(t, fork, "remote", "set-url", "origin", "https://github.com/o/fork.git")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/fork" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"fork": true, "parent": map[string]any{"clone_url": parent, "default_branch": "master"}})
	}))
	t.Cleanup(srv.Close)
	env := map[string]string{"GITHUB_API_URL": srv.URL}
	return fork, forkscope.Resolver{Getenv: func(k string) string { return env[k] }, ListURL: srv.URL + "/fork-of.json"}
}

// In a fork, the sweep repairs only the lines the fork wrote, and leaves every
// file and every line it inherited as it was.
func TestTheSweepInAForkKeepsToTheForksLines(t *testing.T) {
	root, forks := forked(t, http.StatusOK)
	result := commentfix.FixTreeIn(forks, root)

	assert.Empty(t, result.Skipped)
	assert.Equal(t, "package m\n\n// It holds 3 keys.\nvar a int\n", read(t, root, "up.go"))
	both := read(t, root, "both.go")
	assert.Contains(t, both, "// It holds 3 keys.\nvar b int\n", "an inherited line stays")
	assert.NotContains(t, both, "6 locks", "the line the fork wrote is repaired")
}

// A fork whose base cannot be read writes nothing and says why.
func TestTheSweepInAForkWithNoBaseWritesNothing(t *testing.T) {
	root, forks := forked(t, http.StatusInternalServerError)
	result := commentfix.FixTreeIn(forks, root)

	assert.Contains(t, result.Skipped, "500")
	assert.Contains(t, read(t, root, "both.go"), "6 locks")
}

// A repository with no default branch to measure against gives the sweep no
// scope, and a sweep with no scope writes nothing rather than everything.
func TestTheSweepWritesNothingWithoutAMergeBase(t *testing.T) {
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n", "a.go": "package m\n\n// It holds 3 keys.\nvar a int\n"})
	run(t, root, "init", "-q", "-b", "trunk")
	run(t, root, "add", "-A")
	run(t, root, "commit", "-q", "-m", "base")

	result := commentfix.FixTree(root)
	assert.Contains(t, result.Skipped, "no default branch")
	assert.Contains(t, read(t, root, "a.go"), "3 keys")
}
