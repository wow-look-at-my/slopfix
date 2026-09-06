package tombstones

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABlockCommentIsOneFinding(t *testing.T) {
	src := "/*\nThe loader previously read the flag.\n*/\nfunc f() {}\n"
	blocks := AddedBlocks("a.go", src)
	require.NotEmpty(t, blocks)

	hits := Find(blocks, 0)
	require.Len(t, hits, 1)
	assert.Equal(t, "a former state", hits[0].Tell)
}

func TestAHashCommentIsScannedToo(t *testing.T) {
	hits := Find(AddedBlocks("run.sh", "# this used to call the other one\nrun\n"), 0)
	require.Len(t, hits, 1)
	assert.True(t, hits[0].Strippable)
}

func TestAMarkerInsideAStringIsNotAComment(t *testing.T) {
	assert.Empty(t, AddedBlocks("a.go", "s := \"// previously\"\n"))
	assert.Empty(t, AddedBlocks("a.go", "s := `// previously`\n"))
}

func TestFrontmatterAndIndentedCodeAreNotProse(t *testing.T) {
	doc := "---\ntitle: previously\n---\n\nThe loader reads the flag.\n\n    // this used to read it\n"
	assert.Empty(t, Find(AddedBlocks("a.md", doc), 0))
}

func TestAPhraseInBackticksIsALiteralRatherThanAClaim(t *testing.T) {
	assert.Empty(t, Find(AddedBlocks("a.md", "The `previously` rule names a former state.\n"), 0))
}

func TestHitForNameFindsTheLineThatNamesIt(t *testing.T) {
	blocks := AddedBlocks("a.go", "// see TestDarwinStatfsToLinux for the pin\nfunc f() {}\n")
	hit := HitForName(blocks, "TestDarwinStatfsToLinux")

	assert.Equal(t, "TestDarwinStatfsToLinux", hit.Phrase)
	assert.Contains(t, hit.Line, "for the pin")
	assert.True(t, hit.Strippable)
}

func TestHitForNameFallsBackToTheNameItself(t *testing.T) {
	hit := HitForName(nil, "TestDarwinStatfsToLinux")

	assert.Equal(t, "TestDarwinStatfsToLinux", hit.Line)
	assert.False(t, hit.Strippable)
}

func TestIdentifierWordsSplitsOnEveryCharacterASymbolCannotHold(t *testing.T) {
	assert.Equal(t, []string{"see", "readFlag", "and", "flag_name"}, identifierWords("see readFlag() and flag_name."))
}

func TestNoRepositoryMeansNoAnswerRatherThanADeadName(t *testing.T) {
	dir := t.TempDir()
	src := "// see TestDarwinStatfsToLinux for the pin\n"
	assert.Empty(t, DeadReferents(filepath.Join(dir, "a.go"), src, AddedBlocks("a.go", src)))
}

func TestANameTheRepositoryDoesNotDefineIsDead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n"), 0o644))

	path := filepath.Join(dir, "a.go")
	src := "// see TestDarwinStatfsToLinux for the pin\n"
	dead := DeadReferents(path, src, AddedBlocks(path, src))
	if _, err := os.Stat("/usr/bin/rg"); err != nil {
		t.Skip("ripgrep is what answers this, and it is absent")
	}
	assert.Equal(t, []string{"TestDarwinStatfsToLinux"}, dead)
}

func TestANameTheRepositoryDefinesIsAlive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("func TestDarwinStatfsToLinux() {}\n"), 0o644))

	path := filepath.Join(dir, "a.go")
	src := "// see TestDarwinStatfsToLinux for the pin\n"
	assert.Empty(t, DeadReferents(path, src, AddedBlocks(path, src)))
}

func TestRepoRootFindsTheTreeAboveAFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	assert.Equal(t, dir, RepoRoot(filepath.Join(dir, "sub", "a.go")))
}
