package noworkloss

import "testing"

// A session is told to merge the base branch into its PR head, so refusing
// that merge leaves the branch unpushable.
func TestIntegratingCommittedWorkIsAllowedOnACleanTree(t *testing.T) {
	dir := newRepo(t)

	allowed(t, dir, "git merge origin/master")
	allowed(t, dir, "git merge --no-edit FETCH_HEAD")
	allowed(t, dir, "git pull origin master")
	allowed(t, dir, "git pull --no-rebase origin master")
}

// A merge into a tree with uncommitted edits can clobber bytes in no commit,
// so the destruction half preserves the edit rather than refusing.
func TestIntegratingPreservesAndAllowsWithUncommittedWork(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	preserved(t, dir, "git merge origin/master")
	// The line above COMMITTED the edit, so the tree is clean again. Re-dirty
	writeAt(t, dir, "tracked.go", "package a\n// edited again\n")
	preserved(t, dir, "git pull origin master")
}

// Applying a patch authors content in no commit, so it stays refused even on a clean tree.
func TestApplyingAPatchIsStillRefused(t *testing.T) {
	dir := newRepo(t)

	denied(t, dir, "git apply /tmp/change.patch")
	denied(t, dir, "git am /tmp/change.patch")
}

// A staged change counts as outstanding too: the index is not a commit.
func TestMergeAndPullPreserveOverAStagedChange(t *testing.T) {
	// Each spelling gets its own repository. Preservation COMMITS the staged
	// change, so a repeat call in the same tree finds nothing left at risk and
	// is allowed without preserving anything.
	for _, cmd := range []string{"git merge feature", "git pull origin master", "git pull --rebase"} {
		t.Run(cmd, func(t *testing.T) {
			dir := newRepo(t)
			modify(t, dir)
			stage(t, dir)
			preserved(t, dir, cmd)
		})
	}
}

// An untracked scratch file is not something a merge can overwrite: git
// refuses the merge instead.
func TestMergeAndPullIgnoreUntrackedFiles(t *testing.T) {
	dir := newRepo(t)
	untrack(t, dir, "scratch.txt")

	allowed(t, dir, "git merge feature")
	allowed(t, dir, "git pull origin master")
}
