package slopfix_test

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

// A shell comment inside a run: script is judged and repaired. The comment
// gate compares data with a block scalar's # lines blanked, because a # inside
// a scalar is part of the script's string.
func TestALongSentenceInARunScriptCommentIsRepaired(t *testing.T) {
	src := "name: CI\n" +
		"on:\n" +
		"  push:\n" +
		"concurrency:\n" +
		"  group: g\n" +
		"  cancel-in-progress: true\n" +
		"jobs:\n" +
		"  build:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - run: |\n" +
		"          # Building from a directory also asks the module cache for nothing, so the generator that lives in the module needing it never has to complete that module earliest.\n" +
		"          make\n"
	path := ".github/workflows/ci.yml"
	require.Contains(t, findingIDs(slopfix.CheckContent(path, src)), ste.IDSentenceCap, "the control: the comment is over the cap")

	out := slopfix.Fix(slopfix.Request{Content: src, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.NotEqual(t, src, out)
	assert.NotContains(t, findingIDs(slopfix.CheckContent(path, out)), ste.IDSentenceCap, out)
}
