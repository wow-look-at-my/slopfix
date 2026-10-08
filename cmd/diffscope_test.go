package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
)

// diffGoBase and diffGoWork hold a comment number on an unchanged line, and the change adds one.
const diffGoBase = "package p\n\n// Open waits 4 seconds before it starts.\n\n// Close keeps the lock.\n\nfunc Open() {}\n"
const diffGoWork = "package p\n\n// Open waits 4 seconds before it starts.\n\n// Close waits 2 minutes before it stops.\n\nfunc Open() {}\n"

const diffDocBase = "# Doc\n\nAlpha doesn't hold the lock.\n\nBeta holds the lock.\n"
const diffDocWork = "# Doc\n\nAlpha doesn't hold the lock.\n\nBeta doesn't hold the lock.\n"

// diffDocClean changes the same line to a spelling that holds no finding.
const diffDocClean = "# Doc\n\nAlpha doesn't hold the lock.\n\nBeta clears the lock.\n"

// diffRepo builds a work tree with both files committed at their base state.
func diffRepo(t *testing.T) (dir, goPath, docPath string) {
	t.Helper()
	dir = t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	put(t, dir, "open.go", diffGoBase)
	put(t, dir, "doc.md", diffDocBase)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	return dir, filepath.Join(dir, "open.go"), filepath.Join(dir, "doc.md")
}

func diffChanged(t *testing.T, goPath, docPath string) {
	t.Helper()
	put(t, filepath.Dir(goPath), "open.go", diffGoWork)
	put(t, filepath.Dir(docPath), "doc.md", diffDocWork)
}

// selectorMode sets the line-selector flags for one test and restores them.
func selectorMode(t *testing.T, fix, staged bool, rev string) {
	t.Helper()
	oldFix, oldStaged, oldDiff := checkFix, checkStaged, checkDiff
	checkFix, checkStaged, checkDiff = fix, staged, rev
	t.Cleanup(func() { checkFix, checkStaged, checkDiff = oldFix, oldStaged, oldDiff })
}

// assertOnlyLineFiveRepaired proves the changed line's finding is gone and the
// unchanged line's finding stays.
func assertOnlyLineFiveRepaired(t *testing.T, goPath, docPath string) {
	t.Helper()
	goText := readT(t, goPath)
	assert.Contains(t, goText, "waits 4 seconds", "the unchanged line's finding stays")
	assert.NotContains(t, goText, "waits 2 minutes", "the changed line's finding is repaired")

	docText := readT(t, docPath)
	assert.Contains(t, docText, "Alpha doesn't hold the lock.", "the unchanged line's finding stays")
	assert.NotContains(t, docText, "Beta doesn't hold", "the changed line's finding is repaired")
}

func TestDiffSelectorNamesTheChangedLineOfEachFile(t *testing.T) {
	dir, goPath, docPath := diffRepo(t)
	diffChanged(t, goPath, docPath)
	sel, err := forkscope.DiffLines(dir, "HEAD", false)
	require.NoError(t, err)
	assert.True(t, sel.Scope(goPath).Owns(5), "the Go file's changed line is named")
	assert.True(t, sel.Scope(docPath).Owns(5), "the markdown file's changed line is named")

	scoped := slopfix.Fix(slopfix.Request{Path: goPath, Content: diffGoWork, Owned: forkscope.OfLines(5)})
	assert.Contains(t, scoped.Text, "waits 4 seconds", "the unchanged line stays")
	assert.NotContains(t, scoped.Text, "waits 2 minutes", "the owned line is repaired")
}

// `fix --diff HEAD` repairs the changed line alone, over named files.
func TestFixDiffRepairsOnlyTheChangedLine(t *testing.T) {
	t.Serial()
	dir, goPath, docPath := diffRepo(t)
	diffChanged(t, goPath, docPath)
	t.Chdir(dir)
	selectorMode(t, true, false, "HEAD")

	require.NoError(t, runCheck(rootCmd, []string{goPath, docPath}))
	assertOnlyLineFiveRepaired(t, goPath, docPath)
}

// `fix --staged` repairs the changed line alone.
func TestFixStagedRepairsOnlyTheChangedLine(t *testing.T) {
	t.Serial()
	dir, goPath, docPath := diffRepo(t)
	diffChanged(t, goPath, docPath)
	gitRun(t, dir, "add", "-A")
	t.Chdir(dir)
	selectorMode(t, true, true, "")

	require.NoError(t, runCheck(rootCmd, []string{goPath, docPath}))
	assertOnlyLineFiveRepaired(t, goPath, docPath)
}

// `fix --diff HEAD` over a directory walks the tree and keeps to the changed
// lines there too.
func TestFixDiffOverATreeKeepsToTheChangedLines(t *testing.T) {
	t.Serial()
	dir, goPath, docPath := diffRepo(t)
	diffChanged(t, goPath, docPath)
	t.Chdir(dir)
	selectorMode(t, true, false, "HEAD")

	require.NoError(t, runCheck(rootCmd, []string{"."}))
	assertOnlyLineFiveRepaired(t, goPath, docPath)
}

// With no selector, both findings are repaired, which is the behavior the
// selector must not change.
func TestFixWithNoSelectorRepairsEveryLine(t *testing.T) {
	t.Serial()
	dir, goPath, docPath := diffRepo(t)
	diffChanged(t, goPath, docPath)
	t.Chdir(dir)
	selectorMode(t, true, false, "")

	require.NoError(t, runCheck(rootCmd, []string{goPath, docPath}))

	goText := readT(t, goPath)
	assert.NotContains(t, goText, "waits 4 seconds")
	assert.NotContains(t, goText, "waits 2 minutes")
	assert.NotContains(t, readT(t, docPath), "doesn't")
}

// `check --staged` exits nonzero on a staged finding, and zero when the only
// finding sits on a line the staged change did not touch.
func TestCheckStagedFailsOnAChangedFindingAndPassesOtherwise(t *testing.T) {
	t.Serial()
	dir, _, docPath := diffRepo(t)
	put(t, dir, "doc.md", diffDocWork)
	gitRun(t, dir, "add", "doc.md")
	t.Chdir(dir)
	selectorMode(t, false, true, "")

	assert.ErrorIs(t, runCheck(rootCmd, []string{docPath}), errFindings, "a staged finding fails the check")

	put(t, dir, "doc.md", diffDocClean)
	gitRun(t, dir, "add", "doc.md")
	require.NoError(t, runCheck(rootCmd, []string{docPath}), "an unchanged-line finding is not selected, so the check passes")
}

// A selector and the fork resolver are both line scopes, so setting both names
// one line set each.
func TestSelectorFlagsRefuseEachOther(t *testing.T) {
	t.Serial()
	selectorMode(t, false, true, "HEAD")
	assert.Error(t, runCheck(rootCmd, []string{t.TempDir()}), "--staged and --diff cannot both be set")
}
