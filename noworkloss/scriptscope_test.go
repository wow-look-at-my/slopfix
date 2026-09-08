package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A script file the walk runs as a NEW shell is a program. Its variables, its
// working directory and the text it hands another shell are all its own, and
// this hook does not sandbox the programs it starts. varenv_test.go covers the
// operand half of that rule. These cover the halves it was missing: where
// the program stands, and what it could not read at all. Every case pairs the
// script form that must run with the same text typed at top level, which must
// still be refused.

// writeScript puts an executable shell script in dir and returns the command
// that runs it as a fresh shell from dir.
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/bash\n"+body), 0o755))
	return "cd " + dir + " && ./" + name
}

// A build script that cds to its argument and removes paths under it. Every
// operand is static; the directory they land in is not.
func TestAScriptsUnresolvableWorkingDirectoryIsTheProgramsOwnBusiness(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	untrack(t, dir, "scratch.txt")

	cmd := writeScript(t, dir, "bootstrap.bash", `targ="$1"
cd "$targ"
rm -f .gitignore
rm -rf pkg/bootstrap pkg/obj
`)
	allowed(t, dir, cmd)
}

// The control: the same statements typed into the command line are the
// ambiguity this plugin refuses rather than guesses at.
func TestAnUnresolvableWorkingDirectoryStillDeniesInTheCommandText(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	r := denied(t, dir, `cd "$TARG" && rm -f .gitignore`)
	assert.Contains(t, r, "not statically known")
}

// The other control: a script standing somewhere it named STATICALLY is still
// judged, so the walk still sees what it deletes.
func TestAScriptsKnownWorkingDirectoryIsStillJudged(t *testing.T) {
	dir := newRepo(t)
	writeAt(t, dir, "sub/keep.txt", "scratch\n")

	cmd := writeScript(t, dir, "clean.bash", "cd sub\nrm -f keep.txt\n")
	notice := preserved(t, dir, cmd)
	assert.Contains(t, notice, "sub/keep.txt")
}

// A script that hands another shell a command built at run time. The walk
// records a blocker, which used to refuse the whole script.
func TestAScriptsUnreadableInnerShellIsTheProgramsOwnBusiness(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	cmd := writeScript(t, dir, "run-tests.sh", "sh -c \"$1\"\n")
	allowed(t, dir, cmd)
}

// The control: the same `sh -c` in the command text still denies, because
// there the unreadable program IS what the session asked to run.
func TestAnUnreadableInnerShellStillDeniesInTheCommandText(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)

	r := denied(t, dir, `sh -c "$1"`)
	assert.Contains(t, r, "assembled from an expansion")
}

// A script the walk cannot parse is still refused: that blocker describes the
// command that named the file, not the program inside it.
func TestAnUnparseableScriptStillDenies(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.sh"),
		[]byte("#!/bin/bash\nif then fi ((\n"), 0o755))

	r := denied(t, dir, "cd "+dir+" && bash ./broken.sh")
	assert.Contains(t, r, "does not parse as shell")
}

// A script whose blockers are exempt must still be judged on what the walk
// COULD read.
func TestAScriptWithABlockerIsStillJudgedOnWhatItNamesStatically(t *testing.T) {
	dir := newRepo(t)

	cmd := writeScript(t, dir, "mixed.sh", "sh -c \"$1\"\necho generated > tracked.go\n")
	r := denied(t, dir, cmd)
	assert.Contains(t, r, "tracked.go")
}

// `set -e` sets a shell option and binds no name, so it must not turn off
// variable resolution. The redirect below resolves outside the tree.
func TestShellOptionSetDoesNotDisableVariableResolution(t *testing.T) {
	dir := newRepo(t)
	modify(t, dir)
	scratch := t.TempDir()

	allowed(t, dir, `set -e
set +o pipefail
OUT=`+scratch+`/out.log
echo x > "$OUT"`)
}

// The narrowing's own control: the spellings of `set` that really do rebind
// parameters still turn resolution off, so the same redirect denies.
func TestParameterSettingSetStillDisablesVariableResolution(t *testing.T) {
	scratch := t.TempDir()
	for _, form := range []string{"set -- a b", "set a b", "set"} {
		t.Run(form, func(t *testing.T) {
			dir := newRepo(t)
			modify(t, dir)
			r := denied(t, dir, form+`
OUT=`+scratch+`/out.log
echo x > "$OUT"`)
			assert.Contains(t, r, "cannot resolve")
		})
	}
}
