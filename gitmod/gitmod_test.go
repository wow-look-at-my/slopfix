package gitmod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/gitmod"
	"github.com/wow-look-at-my/slopfix/gitmod/gitmodtest"
)

func TestARealSubmoduleIsSkipped(t *testing.T) {
	root := gitmodtest.RepoWithSubmodule(t, "vendored")

	skip, err := gitmod.Skip(root)
	require.NoError(t, err)
	assert.True(t, skip.Contains(gitmod.Resolved(filepath.Join(root, "vendored"))))
}

// A work tree reached through a symlink has names, and the walk comparing
// against this set uses whichever name it was handed. The set answered for a
// single spelling only, so the walk judged the submodule's own files as though
// they were this repository's. Every temporary directory on macOS is such a
// symlink, which is where this surfaced.
func TestASubmoduleIsSkippedThroughASymlinkToItsWorkTree(t *testing.T) {
	root := gitmodtest.RepoWithSubmodule(t, "vendored")
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))

	skip, err := gitmod.Skip(link)
	require.NoError(t, err)
	assert.True(t, skip.Contains(gitmod.Resolved(filepath.Join(link, "vendored"))))
	assert.Equal(t,
		gitmod.Resolved(filepath.Join(root, "vendored")),
		gitmod.Resolved(filepath.Join(link, "vendored")),
		"the two names of one directory have to reduce to one entry")
}

// A path nothing stands at keeps a usable answer, because a caller asking
// about it wants a single rather than an error.
func TestResolvedAnswersForAPathThatIsNotThere(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	assert.Equal(t, missing, gitmod.Resolved(missing))
}

// The forgery. A declaration alone must never exempt a directory, or the check
// is off for anyone who edits a file there.
func TestADeclaredPathThatIsNotAGitlinkIsAnError(t *testing.T) {
	root := gitmodtest.RepoWithFakeSubmodule(t, "src")

	_, err := gitmod.Skip(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gitlink")
	assert.Contains(t, err.Error(), "src")
}

// The path itself cannot reach outside the repository either.
func TestAnEscapingPathIsAnError(t *testing.T) {
	for _, path := range []string{".", "../elsewhere", "/etc"} {
		root := gitmodtest.RepoWithFakeSubmodule(t, "src")
		require.NoError(t, os.WriteFile(filepath.Join(root, ".gitmodules"),
			[]byte("[submodule \"x\"]\n\tpath = "+path+"\n\turl = ./nowhere\n"), 0o644))

		_, err := gitmod.Skip(root)
		require.Error(t, err, "path %q", path)
	}
}

// A repository with no submodules skips nothing, rather than erroring.
func TestNoGitmodulesSkipsNothing(t *testing.T) {
	root := gitmodtest.RepoWithSubmodule(t, "vendored")
	require.NoError(t, os.Remove(filepath.Join(root, ".gitmodules")))

	skip, err := gitmod.Skip(root)
	require.NoError(t, err)
	assert.Zero(t, skip.Len())
}

// Losing the derivation must make the check STRICTER, never laxer: a directory
// that is no repository at all skips nothing and is judged whole.
func TestSomethingThatIsNotARepositorySkipsNothing(t *testing.T) {
	skip, err := gitmod.Skip(t.TempDir())
	require.NoError(t, err)
	assert.Zero(t, skip.Len())
}

// A file argument is answered by the work tree holding it.
func TestAFileResolvesToItsWorkTree(t *testing.T) {
	root := gitmodtest.RepoWithSubmodule(t, "vendored")

	skip, err := gitmod.Skip(filepath.Join(root, ".keep"))
	require.NoError(t, err)
	assert.True(t, skip.Contains(gitmod.Resolved(filepath.Join(root, "vendored"))))
}
