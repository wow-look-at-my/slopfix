package forkscope

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasObject reports whether the object store at dir holds oid. It lists the
// store, because a read of a missing object in a partial clone fetches it.
func hasObject(t *testing.T, dir, oid string) bool {
	t.Helper()
	cmd := exec.Command("git", "cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.Output()
	require.NoError(t, err)
	return slices.Contains(strings.Fields(string(out)), oid)
}

// commitN writes file name with body and commits it in dir.
func commitN(t *testing.T, dir, name, body string) string {
	t.Helper()
	writeT(t, dir, name, body)
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", name)
	return gitT(t, dir, "rev-parse", "HEAD")
}

// A shallow clone of a fork must not pull the parent's whole history to find
// the fork point. The parent knows none of the fork's commits, so a plain
// fetch from it carries every commit and tree it has.
func TestAShallowForkFindsItsBaseWithoutTheParentsHistory(t *testing.T) {
	work := t.TempDir()
	gitT(t, work, "init", "-q", "-b", "main")
	root := commitN(t, work, "doc.md", inherited)
	for i := range 100 {
		commitN(t, work, fmt.Sprintf("old-%03d.md", i), "old\n")
	}
	forkPoint := gitT(t, work, "rev-parse", "HEAD")
	parent := filepath.Join(t.TempDir(), "parent.git")
	gitT(t, work, "clone", "-q", "--bare", work, parent)

	origin := filepath.Join(t.TempDir(), "fork.git")
	gitT(t, work, "clone", "-q", "--bare", parent, origin)
	scratch := filepath.Join(t.TempDir(), "scratch")
	gitT(t, work, "clone", "-q", origin, scratch)
	for i := range 3 {
		commitN(t, scratch, fmt.Sprintf("fork-%d.md", i), "# Fork\n\nThe fork adds this.\n")
	}
	gitT(t, scratch, "push", "-q", origin, "main")

	for i := range 35 {
		commitN(t, work, fmt.Sprintf("new-%03d.md", i), "new\n")
	}
	gitT(t, work, "push", "-q", parent, "main")

	clone := filepath.Join(t.TempDir(), "clone")
	gitT(t, work, "clone", "-q", "--depth=1", "--filter=blob:none", "file://"+origin, clone)
	require.FileExists(t, filepath.Join(clone, ".git", "shallow"))

	srv := forkAPI(t, forkBody("file://"+parent))
	base, err := Resolver{Getenv: forkEnv(srv), ListURL: noList(t)}.Base(clone)
	require.NoError(t, err)
	require.NotNil(t, base)
	assert.Equal(t, forkPoint, base.Commit())
	assert.False(t, hasObject(t, clone, root), "the parent's root commit came over: the fetch took the whole history")

	own, err := base.Lines()
	require.NoError(t, err)
	assert.True(t, own.Whole(filepath.Join(clone, "fork-0.md")))
	assert.False(t, own.Holds(filepath.Join(clone, "old-000.md"), 1, 1))
}

func TestUnbornNamesOnlyAFileNoCommitOrIndexHolds(t *testing.T) {
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	commitN(t, dir, "kept.md", "kept\n")
	commitN(t, dir, "staged-out.md", "gone\n")
	gitT(t, dir, "rm", "-q", "--cached", "staged-out.md")
	writeT(t, dir, "added.md", "added\n")
	gitT(t, dir, "add", "added.md")

	assert.True(t, Unborn(filepath.Join(dir, "new.md")))
	assert.True(t, Unborn(filepath.Join(dir, "deep", "er", "new.md")), "a path under a directory that does not exist yet")
	assert.False(t, Unborn(filepath.Join(dir, "kept.md")))
	assert.False(t, Unborn(filepath.Join(dir, "added.md")), "the index holds it")
	assert.False(t, Unborn(filepath.Join(dir, "staged-out.md")), "HEAD holds it")
	assert.False(t, Unborn(filepath.Join(t.TempDir(), "outside.md")), "no work tree")
	assert.False(t, Unborn(strings.TrimSuffix(dir, "/")))
}
