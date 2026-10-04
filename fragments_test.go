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
