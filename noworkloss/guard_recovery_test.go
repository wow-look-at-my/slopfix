package noworkloss

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// revert replays a commit git holds, as cherry-pick does. commit-tree wraps a
// tree the store already holds and touches no file.
func TestRevertAndCommitTreeAreNotProvenanceRoutes(t *testing.T) {
	dir := newRepo(t)
	allowed(t, dir, "git revert --no-edit HEAD")
	allowed(t, dir, "git commit-tree HEAD^{tree} -p HEAD -m reland")
	allowed(t, dir, `reland=$(git commit-tree "HEAD^{tree}" -p HEAD -m reland) && git checkout -q -B reland "$reland"`)
	modify(t, dir)
	preserved(t, dir, "git revert --no-edit HEAD")
}

func TestGitRmCachedLeavesTheFileAlone(t *testing.T) {
	dir := newRepo(t)
	untrack(t, dir, "scratch.txt")
	allowed(t, dir, "git rm --cached scratch.txt")
	// `git rm` names no provenance route either, so a forced removal.
	preserved(t, dir, "git rm -f scratch.txt")
}

// ---------------------------------------------------------------------------
// Non-git destruction.
// ---------------------------------------------------------------------------

func TestDeniesTruncatingRedirectOntoDirtyFile(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	denied(t, dir, "echo x > tracked.go")
	// An append loses nothing, so the destruction half allows it.
	assert.Empty(t, lossOnly(t, dir, "echo x >> tracked.go"))
	assert.Contains(t, denied(t, dir, "echo x >> tracked.go"), "tracked.go")
}

// `mv` within the tree is not a provenance route either. copyWrites treats a
// source already inside the tree as ordinary refactoring. So the hook only
// preserves the destination's current content.
func TestPreservesAndAllowsMvOverDirtyDestination(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	untrack(t, dir, "src.txt")
	preserved(t, dir, "mv src.txt tracked.go")
	allowed(t, dir, "mv src.txt renamed.txt") // destination does not exist
}

func TestPreservesAndAllowsRmDirectoryContainingUncommittedWork(t *testing.T) {
	dir := newRepo(t)
	untrack(t, dir, "internal/config/env.go")
	preserved(t, dir, "rm -rf internal")
	// Preservation committed that file, so the later spelling needs its own
	untrack(t, dir, "internal/config/other.go")
	preserved(t, dir, "rm -rf .")
}

func TestDeniesTeeAndTruncateOntoDirtyFile(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	denied(t, dir, "echo x | tee tracked.go")
	assert.Empty(t, lossOnly(t, dir, "echo x | tee -a tracked.go"), "appending loses nothing")
	denied(t, dir, "echo x | tee -a tracked.go") // but it is still authoring
	denied(t, dir, "truncate -s 0 tracked.go")
}

// ---------------------------------------------------------------------------
// Staying out of the way.
// ---------------------------------------------------------------------------

func TestIgnoresNonBashToolsAndOtherEvents(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	assert.Empty(t, decideWithEvent(t, "PreToolUse", "Read", dir, "git reset --hard"))
	assert.Empty(t, decideWithEvent(t, "PostToolUse", "Bash", dir, "git reset --hard"))
	unparseable, notices := decide([]byte("not json at all"))
	assert.Empty(t, unparseable)
	assert.Empty(t, notices, "an unparseable payload preserves nothing, so it announces nothing")
}

func TestAllowsDestructiveCommandsOutsideAnyRepository(t *testing.T) {
	plain := t.TempDir()
	writeAt(t, plain, "notes.txt", "hi\n")
	allowed(t, plain, "rm notes.txt")
}

// The prefilter must never shell out for a command that names nothing
// destructive -- that is what keeps the common case cheap.
func TestPrefilterRejectsOrdinaryCommands(t *testing.T) {
	for _, c := range []string{
		"ls -la", "echo hello", "npm test", "go build ./...", "cat file.txt",
	} {
		assert.False(t, mayDestroy(c), "prefilter should skip %q", c)
	}
	for _, c := range []string{
		"git reset --hard", "rm x", "mv a b", "echo x > f", "truncate -s 0 f",
	} {
		assert.True(t, mayDestroy(c), "prefilter must not skip %q", c)
	}
}

func BenchmarkPrefilterMiss(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mayDestroy("npm run build && node dist/index.js --verbose")
	}
}
