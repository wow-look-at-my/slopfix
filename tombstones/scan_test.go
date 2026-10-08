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
	assert.Equal(t, 2, hit.LineNo, "the line counts from one")
	assert.False(t, hit.Strippable)
}

// gitTree makes a repository whose only other file names nothing the cases use.
func gitTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export const other = 1;\n"), 0o644))
	return dir
}

// The scanner call of the code action's comments.ts, as written before its
// inline comment named a dead parameter. The hit sits on the line that holds
// it, counted from one, and the code around the comment names nothing.
func TestADeadNameInAnInlineBlockCommentIsReportedOnItsLine(t *testing.T) {
	path := filepath.Join(gitTree(t), "a.ts")
	src := "function commentOnlyLines(script: string): number[] {\n" +
		"\tconst scanner = ts.createScanner(ts.ScriptTarget.Latest, /* skipTrivia */ false, ts.LanguageVariant.Standard, script);\n" +
		"}\n"
	hits := DeadReferentHits(path, src)
	require.Len(t, hits, 1)
	assert.Equal(t, "skipTrivia", hits[0].Phrase)
	assert.Equal(t, 2, hits[0].LineNo)
}

// The repair takes the whole inline comment, closer included, and leaves the
// call as it would read with no comment at all.
func TestAnInlineBlockCommentEmptiedOfItsDeadNameGoesWhole(t *testing.T) {
	path := filepath.Join(gitTree(t), "a.ts")
	src := "function commentOnlyLines(script: string): number[] {\n" +
		"\tconst scanner = ts.createScanner(ts.ScriptTarget.Latest, /* skipTrivia */ false, ts.LanguageVariant.Standard, script);\n" +
		"}\n"
	repair := Fix(path, src, DefaultMaxCommentLines)
	assert.Equal(t, "function commentOnlyLines(script: string): number[] {\n"+
		"\tconst scanner = ts.createScanner(ts.ScriptTarget.Latest, false, ts.LanguageVariant.Standard, script);\n"+
		"}\n", repair.Text)
	assert.Empty(t, repair.Kept)
	assert.Empty(t, DeadReferentHits(path, repair.Text))
}

// The palette row of the code action's highlight.ts with a dead name added to
// its trailing comment. The string before the comment is code, the hit sits
// on the row, and the repair cuts the sentence back to the comment's own words.
func TestADeadNameInATrailingCommentIsCutAndTheCodeStays(t *testing.T) {
	path := filepath.Join(gitTree(t), "a.ts")
	src := "const PALETTE = new Map<string, string>([\n" +
		"\t['built_in', fg('#ffa657')], // built-in types (string, number) and globals (console, Math). See paletteFor.\n" +
		"]);\n"
	hits := DeadReferentHits(path, src)
	require.Len(t, hits, 1)
	assert.Equal(t, "paletteFor", hits[0].Phrase)
	assert.Equal(t, 2, hits[0].LineNo)

	repair := Fix(path, src, DefaultMaxCommentLines)
	assert.Equal(t, "const PALETTE = new Map<string, string>([\n"+
		"\t['built_in', fg('#ffa657')], // built-in types (string, number) and globals (console, Math).\n"+
		"]);\n", repair.Text)
	assert.Empty(t, repair.Kept)
}

// The sentence span stops at the comment the parse found, on either side.
func TestSentenceAroundStaysInsideTheComment(t *testing.T) {
	for _, c := range []struct {
		line, prose, want string
	}{
		{
			line:  "f(1, /* oldLegacyArg */ false);",
			prose: "     /* oldLegacyArg */        ",
			want:  "f(1, false);",
		},
		{
			line:  "f(1, /* oldLegacyArg */);",
			prose: "     /* oldLegacyArg */  ",
			want:  "f(1,);",
		},
		{
			line:  "x := 1 // see oldLegacyArg",
			prose: "       // see oldLegacyArg",
			want:  "x := 1",
		},
		{
			line:  "/* see oldLegacyArg for the pin",
			prose: "/* see oldLegacyArg for the pin",
			want:  "/* ",
		},
		{
			line:  "f(/* oldLegacyArg. Kept. */ x)",
			prose: "  /* oldLegacyArg. Kept. */   ",
			want:  "f(/* Kept. */ x)",
		},
	} {
		from, to, ok := sentenceAround(c.line, c.prose, "oldLegacyArg", commentOpeners)
		require.True(t, ok, c.line)
		assert.Equal(t, c.want, c.line[:from]+c.line[to:], c.line)
	}
}

func TestRepoRootFindsTheTreeAboveAFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	assert.Equal(t, dir, RepoRoot(filepath.Join(dir, "sub", "a.go")))
}
