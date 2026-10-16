package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// releaseExclude is a path list a release script reads, with comments.
const releaseExclude = "# exclude.txt - paths left out of the snapshot.\n" +
	"#\n" +
	"# Read by a script after it stages the working set. One entry per\n" +
	"# line; blank lines and #-comments ignored.\n" +
	"#   dir/           - path PREFIX from the repo root (the whole directory)\n" +
	"#   **/name/       - a directory with that NAME at any depth\n" +
	"\n" +
	"# Tool configuration.\n" +
	".tools/\n" +
	"**/.tools/\n" +
	"\n" +
	"# Tool docs.\n" +
	"NOTES.md\n" +
	"**/NOTES.md\n"

// Lines that each hold one word are entries of a list, so no repair joins
// them. This holds for any file the prose rules read, whatever its extension.
func TestAnEntryListIsNotJoined(t *testing.T) {
	path := ".github/release-exclude.txt"
	assert.Empty(t, slopfix.CheckContent(path, releaseExclude))
	out := slopfix.Fix(slopfix.Request{Content: releaseExclude, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Equal(t, releaseExclude, out)

	prose := releaseExclude + "The release script reads these entries\none per line.\n"
	assert.NotEmpty(t, slopfix.CheckContent(path, prose), "the control: a text file with prose in it is a document")
}

var unrepairedParagraphs = map[string]string{
	"selector splice": "## Pattern selection\n\n" +
		"`--regex` / `--regex-file` patterns use Go RE2 syntax. Anchoring is forced. Casing is the pattern's own:\n\n" +
		"- **Patterns compile as written** (every pattern is compiled as `\\A(?:pattern)\\z`): matching is case-sensitive unless the pattern opts in with an inline `(?i)`. Short names are sometimes best distinguished by their case. That is the pattern author's call. The caveat: key equality IS case-insensitive, and different packages can legitimately store different casings of the same key. A case-sensitive pattern that matches only one casing rewrites some packages and misses others, so the \"same\" key stops matching itself across the build. When that happens by accident, the run-wide **casing-drift advisory** (below) flags it. When casings really are different parameters, nothing fires. (`--prefix` and `--keys-file` remain case-insensitive: they name keys, not text patterns.)\n",
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
