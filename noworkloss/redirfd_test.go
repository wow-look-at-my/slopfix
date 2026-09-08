package noworkloss

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A redirect is only this hook's business when it can empty a file holding
// content no git object has. Some shapes never can: a device target swallows
// what it is given, and a descriptor other than stdout carries a stream rather
// than the command's output. Each case below pairs the shape that must pass
// with the shape that must still be refused.

// The reported incident. Nothing here puts a file at risk, and the target
// needs no working directory to resolve.
func TestStderrDiscardInsideAnUnresolvableSubshellIsAllowed(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	untrack(t, dir, "scratch.txt")

	allowed(t, dir, `for d in a b; do (cd "$d" && git status -sb 2>/dev/null); done`)
}

// The control: STDOUT into an unresolvable directory is a genuine ambiguity,
// because the target may be a file the tree holds.
func TestStdoutIntoAnUnresolvableDirectoryStillDenies(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	r := denied(t, dir, `for d in a b; do (cd "$d" && git status -sb > status.log); done`)
	assert.Contains(t, r, "status.log")
}

// Every device spelling, at stdout and at stderr, over a tree dirty enough
// that a real truncation would be refused.
func TestDeviceTargetsAreNeverLosable(t *testing.T) {
	for _, cmd := range []string{
		"echo x > /dev/null",
		"echo x 2>/dev/null",
		"echo x > /dev/stdout",
		"echo x 2> /dev/stderr",
		"echo x > /dev/tty",
		"echo x > /dev/fd/3",
		"echo x &> /dev/null",
		"git status 2>&1",
	} {
		t.Run(cmd, func(t *testing.T) {
			dir := newRepo(t)
			modify(t, dir)
			untrack(t, dir, "scratch.txt")
			allowed(t, dir, cmd)
		})
	}
}

// The control every device case rests on: the same redirect aimed at a dirty
// tracked file is refused by name.
func TestStdoutIntoATrackedFileStillDenies(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	r := denied(t, dir, "echo x > tracked.go")
	assert.Contains(t, r, "tracked.go")
}

// A stderr capture into a file inside the tree is refused by the provenance
// half, which quotes the descriptor the reader wrote.
func TestStderrIntoATrackedFileIsRefusedAsAWriteRatherThanALoss(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	r := denied(t, dir, "echo x 2> tracked.go")
	assert.Contains(t, r, "tracked.go")
	assert.Contains(t, r, "2> tracked.go")
	assert.Contains(t, r, useTheTools)
}

// Both halves, asked separately, on stdout and on stderr. This is the pair
// the whole change turns on: stdout into a dirty tracked file is content this
// hook must save beforehand, and stderr into the same file is not.
func TestOnlyAStdoutRedirectIsAWorkLossFinding(t *testing.T) {
	stdoutRepo := newRepo(t)
	modify(t, stdoutRepo)
	reason, notices := lossOnlyNotices(t, stdoutRepo, "echo x > tracked.go")
	require.Empty(t, reason)
	require.NotEmpty(t, notices, "a stdout truncation of a dirty tracked file must be preserved first")
	assert.Contains(t, notices[0], "tracked.go")

	stderrRepo := newRepo(t)
	modify(t, stderrRepo)
	reason, notices = lossOnlyNotices(t, stderrRepo, "echo x 2> tracked.go")
	assert.Empty(t, reason)
	assert.Empty(t, notices, "stderr puts no file the tree holds at risk, so nothing is preserved for it")
}
