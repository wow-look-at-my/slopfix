package slopfix_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// A comment that trips both the ratio check and the sentence cap. Both
// repairs settle in one fix, and a second fix changes nothing.
func TestTheRatioAndTheSentenceCapSettleTogether(t *testing.T) {
	src := "package demo\n\n" +
		"// The loader reads every cached manifest from the shared store and rebuilds the\n" +
		"// index of plugin hooks for each workspace the user opens in the editor during\n" +
		"// startup of the session. It holds a lock while it reads. It drops the lock\n" +
		"// before it writes the index, so a second editor never waits on the first.\n" +
		"func Load() {}\n"
	ids := findingIDs(slopfix.CheckContent("demo.go", src))
	require.Contains(t, ids, commentfix.IDLength)
	require.Contains(t, ids, ste.IDSentenceCap)

	once := slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	twice := slopfix.Fix(slopfix.Request{Content: once.Text, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.Equal(t, once.Text, twice.Text)
	assert.Empty(t, quoted(slopfix.CheckContent("demo.go", once.Text)), once.Text)
	assert.Contains(t, once.Text, "// The loader reads every cached manifest from the shared store.", once.Text)
}

// The sentence cap reaches a code comment as its own check: a short comment
// over a long function still reports a sentence past the cap.
func TestCheckReportsALongSentenceInAComment(t *testing.T) {
	src := "package demo\n\n" +
		"// The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user on each start of a session in the editor window.\n" +
		"func Load() {\n" + strings.Repeat("\tstep()\n", 60) + "}\n"
	var found []ste.Finding
	for _, f := range slopfix.CheckContent("demo.go", src) {
		if f.ID == ste.IDSentenceCap {
			found = append(found, f)
		}
	}
	require.Len(t, found, 1)
	assert.False(t, ste.ByHand(found[0].Fix))
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go"}).Text
	assert.Contains(t, out, "// The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user.", out)
	assert.Contains(t, strings.ReplaceAll(out, "\n// ", " "), "This happens on each start of a session in the editor window.", out)
	assert.Empty(t, quoted(slopfix.CheckContent("demo.go", out)), out)
}

// A JSDoc sentence whose first division leaves a second sentence still over
// the cap divides again, until every sentence holds.
func TestAColonDivisionThatLeavesALongRestDividesAgain(t *testing.T) {
	src := "/**\n" +
		" * On the default branch the order is the release number, not the tip of the branch: a run that a later commit superseded is still the newest release of a plugin that the later run took from a cache and never published.\n" +
		" */\n" +
		"export function order(): void {\n" + strings.Repeat("\tstep();\n", 60) + "}\n"
	path := "orphan-release/src/index.ts"
	require.Contains(t, findingIDs(slopfix.CheckContent(path, src)), ste.IDSentenceCap, "the control: the comment is over the cap")

	out := slopfix.Fix(slopfix.Request{Content: src, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assertEverySentenceUnderCap(t, path, out)
	assert.Empty(t, quoted(slopfix.CheckContent(path, out)), out)
	assert.Contains(t, strings.ReplaceAll(out, "\n * ", " "), "never published.", out)
}

// A justfile comment whose sentence holds a code span with dots, a dash
// aside and a parenthesis is still divided, at the dashes.
func TestALongJustfileCommentWithADashAsideIsRepaired(t *testing.T) {
	src := "build:\n" +
		"\t# TypeScript standard libs. Only the `/// <reference lib=\"...\" />` closure of the libs\n" +
		"\t# the action can select -- lib.es2022.d.ts always, and lib.dom.d.ts +\n" +
		"\t# lib.dom.iterable.d.ts when a step passes dom: true -- seeded with any lib the staged\n" +
		"\t# type packages reference (@types/node pulls in a couple of esnext.* libs).\n" +
		"\tnpm run build\n"
	path := "typescript/justfile"
	require.Contains(t, findingIDs(slopfix.CheckContent(path, src)), ste.IDSentenceCap, "the control: the comment is over the cap")

	out := slopfix.Fix(slopfix.Request{Content: src, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.NotEqual(t, src, out)
	assertEverySentenceUnderCap(t, path, out)
	assert.Empty(t, quoted(slopfix.CheckContent(path, out)), out)
	assert.Contains(t, out, "\t# ", out)
	assert.Contains(t, out, "\tnpm run build\n", out)
}

// assertEverySentenceUnderCap fails on each sentence of out that Check reads
// past the cap under path.
func assertEverySentenceUnderCap(t *testing.T, path, out string) {
	t.Helper()
	for _, f := range slopfix.CheckContent(path, out) {
		assert.NotEqual(t, ste.IDSentenceCap, f.ID, "sentence over the cap after fix: %s\n%s", f.Detail, out)
	}
}

// A sentence built around a colon, and a sentence held together by a dash
// aside, each divide under the cap in a Go comment. One fix leaves nothing
// for check, and a second fix changes nothing.
func TestAColonAndADashAsideDivideInAGoComment(t *testing.T) {
	for _, c := range []struct{ comment, want string }{
		{
			"// With the server's prefetch config off, which is the default, the client's\n" +
				"// flags change nothing: a batch carries exactly the requested keys, even from\n" +
				"// a client that still sets prefetch on the request its build is blocked on.\n",
			"the client's flags change nothing. A batch carries exactly the requested keys,",
		},
		{
			"// Then every client it refuses rebuilds anyway -- having earliest paid for the round\n" +
				"// trip -- so a cache that sheds is worse than no cache at all.\n",
			"rebuilds anyway, having earliest paid for the round trip. A cache that sheds is worse than no cache at all.",
		},
	} {
		// A long body keeps comments/length from cutting the comment, so the sentence repair is what runs.
		src := "package demo\n\n" + c.comment + "func Demo() {\n" + strings.Repeat("\tstep()\n", 60) + "}\n"
		require.Contains(t, findingIDs(slopfix.CheckContent("demo_test.go", src)), ste.IDSentenceCap, "the control: the sentence is over the cap")

		once := slopfix.Fix(slopfix.Request{Content: src, Path: "demo_test.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
		twice := slopfix.Fix(slopfix.Request{Content: once.Text, Path: "demo_test.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
		assert.NotEqual(t, src, once.Text)
		assert.Equal(t, once.Text, twice.Text)
		assert.Empty(t, quoted(slopfix.CheckContent("demo_test.go", once.Text)), once.Text)
		assert.Contains(t, strings.ReplaceAll(once.Text, "\n// ", " "), c.want, once.Text)
	}
}

// A shell comment inside a run: script is judged and repaired. The comment
// gate compares data with a block scalar's # lines blanked, because a # inside
// a scalar is part of the script's string.
func TestALongSentenceInARunScriptCommentIsRepaired(t *testing.T) {
	src := testALongSentenceInARunScriptCommentIsRepairedYml
	path := ".github/workflows/ci.yml"
	require.Contains(t, findingIDs(slopfix.CheckContent(path, src)), ste.IDSentenceCap, "the control: the comment is over the cap")

	out := slopfix.Fix(slopfix.Request{Content: src, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.NotEqual(t, src, out)
	assert.NotContains(t, findingIDs(slopfix.CheckContent(path, out)), ste.IDSentenceCap, out)
}

// A division that restates an identifier subject keeps its case. A capital
// makes a name nothing defines, and the dead-name check fails.
func TestADivisionRestatesAnIdentifierSubjectWithItsCase(t *testing.T) {
	src := "package demo\n\n" +
		"// foreignJobs answers each job of content that runs on a foreign hosted label\n" +
		"// in a private repository, and that the repair can keep to a public one.\n" +
		"func foreignJobs() {\n" + strings.Repeat("\tstep()\n", 60) + "}\n\nfunc step() {}\n"
	require.Contains(t, findingIDs(slopfix.CheckContent("demo.go", src)), ste.IDSentenceCap, "the control: the comment is over the cap")

	out := slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.NotContains(t, out, "ForeignJobs", out)
	assert.Contains(t, strings.ReplaceAll(out, "\n// ", " "), ". foreignJobs also answers", out)
	assert.Empty(t, quoted(slopfix.CheckContent("demo.go", out)), out)
}

//go:embed testdata/test-a-long-sentence-in-a-run-script-comment-is-repaired.yml
var testALongSentenceInARunScriptCommentIsRepairedYml string
