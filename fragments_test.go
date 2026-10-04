package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// releaseExclude is the head of splat's .github/release-exclude.txt, a path
// list the release script reads.
const releaseExclude = "# release-exclude.txt - paths EXCLUDED from the release snapshot.\n" +
	"#\n" +
	"# Consumed by .github/scripts/build-release-tree.sh after it stages the\n" +
	"# flattened tracked working set (submodule trees included). One entry per\n" +
	"# line; blank lines and #-comments ignored.\n" +
	"#   dir/           - path PREFIX from the repo root (the whole directory)\n" +
	"#   **/name/       - a directory with that NAME at any depth\n" +
	"\n" +
	"# AI dev-tooling configuration (contains personal machine paths).\n" +
	".claude/\n" +
	"**/.claude/\n" +
	"\n" +
	"# AI dev-tooling docs (all four apps).\n" +
	"CLAUDE.md\n" +
	"**/CLAUDE.md\n"

// A text file a program reads line by line is not prose, so no rule reads it
// and no repair joins its entries.
func TestAPathListIsNoDocument(t *testing.T) {
	path := ".github/release-exclude.txt"
	assert.Empty(t, slopfix.CheckContent(path, releaseExclude))
	out := slopfix.Fix(slopfix.Request{Content: releaseExclude, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Equal(t, releaseExclude, out)

	prose := releaseExclude + "The release script reads these entries\none per line.\n"
	assert.NotEmpty(t, slopfix.CheckContent(path, prose), "the control: a text file with prose in it is a document")
}

// generatedRegion is the data layouts region of splat-vulkan's README.md,
// which a cmake target rewrites.
const generatedRegion = "<!-- BEGIN GENERATED: data layouts (cmake --build build --target layouts-docs) -->\n" +
	"\n" +
	"Format versions: `kFrameParamsVersion` = 4, `kCubeBufLayoutVersion` = 4, `kCubeIdPageSize` = 65536 (cube count unbounded - pack capacity demand-grows from a 65536-cube floor), cube-edge auto candidates {0.5, 1, 2, 3, 4, 6, 8} m (target budget 49152 cubes).\n" +
	"The three buffers hold\n" +
	"every cube the pack has in order to draw it.\n" +
	"\n" +
	"<!-- END GENERATED: data layouts -->\n"

// A region a program writes belongs to the program, so no rule reads it and no
// repair rewrites it.
func TestAGeneratedRegionIsLeftToItsProgram(t *testing.T) {
	doc := "# Data layouts\n\n" + generatedRegion
	assert.Empty(t, slopfix.CheckContent("README.md", doc))
	out := slopfix.Fix(slopfix.Request{Content: doc, Path: "README.md", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	assert.Equal(t, doc, out)

	open := strings.ReplaceAll(strings.ReplaceAll(doc, "BEGIN GENERATED", "BEGIN"), "END GENERATED", "END")
	assert.NotEmpty(t, slopfix.CheckContent("README.md", open), "the control: the same text outside a region has findings")
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
