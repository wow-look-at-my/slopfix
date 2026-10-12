package noworkloss

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRepoWithSubmodule returns a superproject that records sub at a single
// commit, and the path of that submodule's working tree.
func newRepoWithSubmodule(t *testing.T) (dir, sub string) {
	t.Helper()
	dir = newRepo(t)
	lib := newRepo(t)
	git(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", lib, "sub")
	git(t, dir, "commit", "-qm", "add sub")
	sub = dir + "/sub"
	git(t, sub, "config", "user.email", "guard@example.com")
	git(t, sub, "config", "user.name", "Guard")
	return dir, sub
}

// moveSubmodule commits inside the submodule, so its HEAD leaves the gitlink.
func moveSubmodule(t *testing.T, sub string) {
	t.Helper()
	writeAt(t, sub, "lib.go", "package lib\n")
	git(t, sub, "add", "-A")
	git(t, sub, "commit", "-qm", "move")
}

// A submodule at a commit other than the gitlink holds nothing uncommitted.
// The commit lives in the submodule's own history, so no verb here loses it.
func TestSubmoduleHashMoveIsNotAtRisk(t *testing.T) {
	dir, sub := newRepoWithSubmodule(t)
	moveSubmodule(t, sub)
	git(t, dir, "branch", "other")
	head := gitOutput(t, dir, "rev-parse", "HEAD")

	for _, c := range []string{
		"git reset --hard",
		"git checkout other",
		"git switch other",
		"git restore .",
		"git checkout -- .",
	} {
		reason, notices := lossOnlyNotices(t, dir, c)
		assert.Empty(t, reason, "a moved gitlink loses nothing: %q", c)
		assert.Empty(t, notices, "a moved gitlink needs no preservation: %q", c)
	}
	assert.Equal(t, head, gitOutput(t, dir, "rev-parse", "HEAD"), "nothing may be committed for a moved gitlink")

	// A staged gitlink names a commit the submodule still holds.
	git(t, dir, "add", "sub")
	reason, notices := lossOnlyNotices(t, dir, "git reset --hard")
	assert.Empty(t, reason)
	assert.Empty(t, notices)
}

// Uncommitted content inside the submodule is still work at risk.
func TestSubmoduleWithUncommittedContentIsAtRisk(t *testing.T) {
	for name, dirty := range map[string]func(t *testing.T, sub string){
		"modified":  func(t *testing.T, sub string) { writeAt(t, sub, "tracked.go", "package a\n// edited\n") },
		"untracked": func(t *testing.T, sub string) { writeAt(t, sub, "new.go", "package a\n") },
	} {
		t.Run(name, func(t *testing.T) {
			dir, sub := newRepoWithSubmodule(t)
			moveSubmodule(t, sub)
			dirty(t, sub)
			st := probeDir(dir)
			require.NoError(t, st.err)
			assert.Contains(t, st.tracked, "sub")
		})
	}
}
