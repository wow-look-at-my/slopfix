package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

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

// userscriptHeader is the metadata block of tampermonkey's
// actions-step-colorizer.user.ts. Tampermonkey parses it, so it is no prose.
const userscriptHeader = "// ==UserScript==\n" +
	"// @name         GitHub Actions Step Status Colorizer\n" +
	"// @version      1.0\n" +
	"// @description  Colorize the step status icons in GitHub Actions workflow runs\n" +
	"// @author       mhaynie\n" +
	"// @match        https://github.com/*/actions/runs/*\n" +
	"// @grant        GM_addStyle\n" +
	"// @run-at       document-end\n" +
	"// ==/UserScript==\n"

// Every rule leaves a userscript metadata block as written, every line of it.
func TestFixKeepsAUserscriptMetadataBlock(t *testing.T) {
	src := userscriptHeader + "\nGM_addStyle(`\n\t.CheckStep .octicon-skip {\n\t\tcolor: #d29922 !important;\n\t}\n`);\n"
	for _, path := range []string{"src/GitHub/actions-step-colorizer.user.ts", "src/GitHub/actions-step-colorizer.user.js"} {
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
