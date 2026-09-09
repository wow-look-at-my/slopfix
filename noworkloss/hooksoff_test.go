package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installRefusingHooks writes a hook that fails for every name preservation
// can reach: the commit path, the ref update, and the push.
func installRefusingHooks(t *testing.T, dir string) {
	t.Helper()
	hooks := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooks, 0o755))
	for _, name := range []string{"pre-commit", "commit-msg", "post-commit", "reference-transaction", "pre-push"} {
		p := filepath.Join(hooks, name)
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\necho refused >&2\nexit 1\n"), 0o755))
	}
}

// A repository hook belongs to the user and may refuse or hang. Preservation
// must not depend on it, so the content still lands on the branch.
func TestPreservationIgnoresRepositoryHooks(t *testing.T) {
	dir := newRepo(t)
	installRefusingHooks(t, dir)
	untrack(t, dir, "scratch.txt")

	notice := preserved(t, dir, "rm scratch.txt")
	assert.Contains(t, notice, "scratch.txt")

	refs := listPreservationRefs(t, dir)
	require.Len(t, refs, 1)
	assert.Equal(t, "scratch", gitOutput(t, dir, "show", refs[0]+":scratch.txt"))
}

// The push half of preservation: a pre-push hook that refuses must not stop
// the commit from reaching the remote.
func TestPreservationPushIgnoresPrePushHook(t *testing.T) {
	dir := remoteRepo(t)
	installRefusingHooks(t, dir)
	writeAt(t, dir, "scratch.txt", "scratch\n")

	notice := preserved(t, dir, "rm scratch.txt")
	assert.NotContains(t, notice, "The push failed")

	head := gitOutput(t, dir, "rev-parse", "HEAD")
	assert.Equal(t, head, gitOutput(t, dir, "rev-parse", "origin/master"))
}
