package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// than a line at a time.
func TestABlockCommentIsOneUnit(t *testing.T) {
	src := "/*\nThe loader previously read the flag.\n*/\nfunc f() {}\n"
	blocks := AddedBlocks("a.go", src)
	require.Len(t, blocks, 1)

	repair := Fix("a.go", src, DefaultMaxCommentLines)
	assert.True(t, repair.Changed)
	assert.NotContains(t, repair.Text, "previously")
	assert.Contains(t, repair.Text, "func f() {}")
}

func TestAHashCommentIsRewrittenToo(t *testing.T) {
	repair := Fix("run.sh", "# this used to call the other one\nrun\n", DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.Equal(t, "run\n", repair.Text)
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

func TestOwnNamesSplitsOnEveryCharacterASymbolCannotHold(t *testing.T) {
	assert.Equal(t, []string{"see", "readFlag", "and", "flag_name"}, ownNames("see readFlag() and flag_name."))
}

// Each name here belongs to another namespace, another owner, a file, an
// address or a placeholder, so none is a symbol this repository must define.
func TestOwnNamesKeepsOnlyNamesOfThisRepository(t *testing.T) {
	for _, prose := range []string{
		"Module paths are case-encoded per module#EscapePath.",
		"Fix math32.wrongCase to the correct package.",
		"It joins the comment the way x/mod's setIndirect joins it.",
		"The fork's isNumericType excludes complex.",
		"Semantics mirror xattr_windows.go here.",
		"Spec: https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU",
		"The test framework calls TestXxx and BenchmarkXxx.",
		"Filename constraints (foo_windows.go) are platform only.",
	} {
		for _, name := range ownNames(prose) {
			assert.False(t, isCandidate(name), "%q in %q", name, prose)
		}
	}
	assert.Contains(t, ownNames("see parseLegacyFlag for the rule"), "parseLegacyFlag")
}

// A line that continues a sentence from the line above never strips whole,
// because the strip would leave half a sentence behind.
func TestAWrappedSentenceLineIsNotStrippable(t *testing.T) {
	src := "package p\n\nfunc f() {\n\t// A tag-excluded file is never compiled. A match\n\t// error means it cannot classify (see fileMatchesBuild).\n\tf()\n}\n"
	hit := HitForName(AddedBlocks("p.go", src), "fileMatchesBuild")
	assert.False(t, hit.Strippable)
	whole := "package p\n\nfunc f() {\n\t// A tag-excluded file is never compiled.\n\t// See fileMatchesBuild for the rule.\n\tf()\n}\n"
	assert.True(t, HitForName(AddedBlocks("p.go", whole), "fileMatchesBuild").Strippable)
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

// A name that only sits inside a longer identifier is not defined, whether the
// index or a ripgrep probe answers.
func TestANameInsideALongerIdentifierIsDead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("func eventBeforeIdleHook() {}\n"), 0o644))

	path := filepath.Join(dir, "a.go")
	src := "// BeforeIdleHook routes the worker\n"
	assert.Equal(t, []string{"BeforeIdleHook"}, DeadReferents(path, src, AddedBlocks(path, src)))
}

// The code around an inline comment is never a referent. The members of an
// imported module name nothing this repository must define.
func TestCodeAroundAnInlineCommentIsNotAReferent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export const other = 1;\n"), 0o644))

	path := filepath.Join(dir, "a.ts")
	src := "import ts from 'typescript';\n" +
		"const scanner = ts.createScanner(ts.ScriptTarget.Latest, /* */ false, ts.LanguageVariant.Standard, script);\n"
	assert.Empty(t, DeadReferents(path, src, AddedBlocks(path, src)))
	assert.Empty(t, DeadReferentHits(path, src))
}

// A string literal before a trailing comment is code, so its spelling is
// never a referent.
func TestAStringBeforeATrailingCommentIsNotAReferent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export const other = 1;\n"), 0o644))

	path := filepath.Join(dir, "a.ts")
	src := "const rules = [\n" +
		"\t['built_in', fg('#ffa657')], // built-in types (string, number) and globals (console, Math)\n" +
		"];\n"
	assert.Empty(t, DeadReferents(path, src, AddedBlocks(path, src)))
	assert.Empty(t, DeadReferentHits(path, src))
}

// A trailing comment that names a dead symbol itself is still reported, and
// the hit sits on the comment's line.
func TestATrailingCommentNamingADeadSymbolIsReported(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export const other = 1;\n"), 0o644))

	path := filepath.Join(dir, "a.ts")
	src := "const x = 1;\n" +
		"const y = 2; // see parseLegacyFlag for the pin\n"
	blocks := AddedBlocks(path, src)
	assert.Equal(t, []string{"parseLegacyFlag"}, DeadReferents(path, src, blocks))
	hit := HitForName(blocks, "parseLegacyFlag")
	assert.Equal(t, 1, hit.LineNo)
	assert.False(t, hit.Strippable)
}

// Document prose that grok-build's check reported: emphasis underscores and product names.
func TestDocumentEmphasisAndProductNamesAreNotReferents(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n"), 0o644))

	path := filepath.Join(dir, "a.md")
	src := "Treat it as _already established_ by the user.\n\n" +
		"The proxy speaks WebSockets and HTTP.\n\n" +
		"Copyright (c) PCRE2Project and its contributors.\n"
	assert.Empty(t, DeadReferents(path, src, AddedBlocks(path, src)))
}

// A bare snake_case name in a document still names a symbol.
func TestDocumentSnakeCaseNameIsStillJudged(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n"), 0o644))

	path := filepath.Join(dir, "a.md")
	src := "Set legacy_retry_flag to turn it off.\n"
	assert.Equal(t, []string{"legacy_retry_flag"}, DeadReferents(path, src, AddedBlocks(path, src)))
}

// A document paragraph places its lines, so the repair that cuts the naming
// sentence can reach them.
func TestADocumentParagraphPlacesItsLines(t *testing.T) {
	blocks := AddedBlocks("a.md", "M parking is the stub_mutex design.\n\nThe cache reads it.\n")
	require.Len(t, blocks, 2)
	require.Len(t, blocks[0].Pure, len(blocks[0].LineNos))
	assert.Equal(t, []bool{true}, blocks[0].Pure)

	hit := HitForName(blocks, "stub_mutex")
	assert.Equal(t, 0, hit.LineNo)
}

func TestRepoRootFindsTheTreeAboveAFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	assert.Equal(t, dir, RepoRoot(filepath.Join(dir, "sub", "a.go")))
}
