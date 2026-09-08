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
	assert.True(t, skip.Contains(filepath.Join(root, "vendored")))
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
	assert.True(t, skip.Contains(filepath.Join(root, "vendored")))
}
