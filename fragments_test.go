package slopfix_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

//go:embed testdata/release-exclude.md
var releaseExclude string

// A wrapped run of one-word lines is a finding, and no repair joins the
// entries. In markdown each becomes a list item.
func TestAnEntryRunInMarkdownBecomesAList(t *testing.T) {
	src := ".claude/\n**/.claude/\nCLAUDE.md\n"
	require.NotEmpty(t, slopfix.CheckContent("paths.md", src))
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "paths.md", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Equal(t, "- .claude/\n- **/.claude/\n- CLAUDE.md\n", out)
	assert.Empty(t, slopfix.CheckContent("paths.md", out))
}

// A text file a program reads keeps every entry a bare line: the repair parts
// the entries with blank lines, which such a file ignores.
func TestAnEntryRunInATextFileKeepsBareLines(t *testing.T) {
	path := ".github/release-exclude.txt"
	require.NotEmpty(t, slopfix.CheckContent(path, releaseExclude))
	out := slopfix.Fix(slopfix.Request{Content: releaseExclude, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Empty(t, slopfix.CheckContent(path, out))
	assert.Equal(t, nonBlank(releaseExclude), nonBlank(out), "every line the program reads survives")
	assert.Contains(t, out, ".tools/\n\n**/.tools/\n")
}

// nonBlank lists the lines of text that hold something.
func nonBlank(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// Prose after the entries is a document too, and joins.
func TestProseAfterEntriesStillJoins(t *testing.T) {
	prose := releaseExclude + "The release script reads these entries\none per line.\n"
	out := slopfix.Fix(slopfix.Request{Content: prose, Path: ".github/release-exclude.txt", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Contains(t, out, "The release script reads these entries one per line.\n")
}

var unrepairedParagraphs = map[string]string{
	"selector splice": embeddedMd,
	"selector length": "**Relationship to `check-output`** (see [Leak gates](#leak-gates-check-source-and-check-output)): the self-check is the *automatic, post-rewrite* gate proving the keys *this run rewrote* are gone from the files. *this run processed*. `check-output` is the *standalone* end-of-build gate you point at ANY artifact (the shipped executable, third-party data) and judge against the *full selector* (every pattern, matched or not). Use both: self-check guards the rewrite itself, `check-output` guards everything the rewrite never saw.\n",
	"frontend length": "# example.wasm\n\n" +
		"The Go frontend ([example.ir](https://example.com/example.ir), vendored as the `.ir/` git submodule) lexes, parses, type-checks, and lowers source to a fully-resolved IR. This repo translates that IR into a WebAssembly module.\n",
}

// What check reports in these paragraphs, fix repairs.
func TestFixRepairsWhatCheckReportsInAReadme(t *testing.T) {
	for name, doc := range unrepairedParagraphs {
		out := slopfix.Fix(slopfix.Request{Content: doc, Path: "README.md", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
		for _, f := range slopfix.CheckContent("README.md", out) {
			assert.NotContains(t, []string{"ste/comma-splice", "ste/sentence-length"}, f.ID, "%s: %s\n%s", name, f, out)
		}
	}
}

// hardlinkSource is basicopy's handleMultiLink doc comment. Its second
// paragraph names what the code did before, inside an aside.
const hardlinkSource = "package engine\n\n" +
	"// handleMultiLink handles a single path to a multiply-linked source inode. The\n" +
	"// earliest path becomes the primary: it is copied, or, when its destination\n" +
	"// is already up to date, the existing destination file is adopted as-is. Every\n" +
	"// later path to the same inode is hardlinked to the primary's destination after\n" +
	"// all copies finish, unless it already IS the adopted primary's inode (nothing to do).\n" +
	"//\n" +
	"// Adopting an unchanged primary (rather than skipping it before the hardlink\n" +
	"// bookkeeping, as this used to) matters for incremental runs: without a\n" +
	"// recorded primary, a secondary missing from the destination was recopied as an\n" +
	"// independent duplicate, silently losing the hardlink structure and doubling\n" +
	"// the stored data.\n" +
	"func (r *runner) handleMultiLink(root *srcRoot, srcPath, dstPath string, key string) {\n" +
	"\tif p, seen := r.hardlinkMap[key]; seen {\n" +
	"\t\tr.hardlinks = append(r.hardlinks, linkEnt{target: p.dst, dst: dstPath})\n" +
	"\t\treturn\n" +
	"\t}\n" +
	"\tr.hardlinkMap[key] = hlPrimary{dst: dstPath}\n" +
	"\tr.enqueueFile(root, srcPath, dstPath)\n" +
	"}\n"

// userscriptHeader is the metadata block of a userscript. The browser
// extension parses it, so it is no prose.
const userscriptHeader = "// ==UserScript==\n" +
	"// @name         Example Status Colorizer\n" +
	"// @version      1.0\n" +
	"// @description  Colorize the status icons on an example page\n" +
	"// @author       example\n" +
	"// @match        https://example.com/*/runs/*\n" +
	"// @grant        GM_addStyle\n" +
	"// @run-at       document-end\n" +
	"// ==/UserScript==\n"

// Every rule leaves a userscript metadata block as written, every line of it.
func TestFixKeepsAUserscriptMetadataBlock(t *testing.T) {
	src := userscriptHeader + "\nGM_addStyle(`\n\t.CheckStep .octicon-skip {\n\t\tcolor: #d29922 !important;\n\t}\n`);\n"
	for _, path := range []string{"src/example/status-colorizer.user.ts", "src/example/status-colorizer.user.js"} {
		out := slopfix.Fix(slopfix.Request{Content: src, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
		assert.Equal(t, src, out, path)
		for _, f := range slopfix.CheckContent(path, src) {
			assert.Greater(t, f.Line, strings.Count(userscriptHeader, "\n"), "%s: %s", path, f)
		}
	}
}

// The tombstone sits inside an aside, so its sentence goes whole, and the
// paragraph break it leaves goes with it. No fragment of the sentence stays.
func TestFixLeavesNoFragmentOfASentenceItCuts(t *testing.T) {
	out := slopfix.Fix(slopfix.Request{Content: hardlinkSource, Path: "hardlink.go", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text

	assert.NotContains(t, out, "used to")
	assert.NotContains(t, out, "bookkeeping, as")
	assert.NotContains(t, out, "Adopting")
	assert.Contains(t, out, ".\nfunc (r *runner) handleMultiLink", "the comment ends on a whole sentence")
	assert.NotContains(t, out, "//\nfunc", "the break before the cut paragraph goes too")
	assert.Equal(t, strings.Count(out, "("), strings.Count(out, ")"), "every aside closes")
}

//go:embed testdata/embedded.md
var embeddedMd string
