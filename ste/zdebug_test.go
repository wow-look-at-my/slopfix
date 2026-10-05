package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestZDebug(t *testing.T) {
	var b strings.Builder
	for _, src := range []string{
		"A trailing run of turns that each made the exact same tool call, spaced close enough together that no real event or scheduled wakeup can plausibly explain the repeat.",
		"The closer is not prose: left in the text a rewrite wraps it into the middle of the comment, and dropped it leaves the block open and the file unparseable.",
	} {
		masked := checkMask(src)
		whole := syntax.Parse(masked, nil)
		fmt.Fprintf(&b, "\n== %s\n%s\n", src, whole.Outline())
		for _, c := range candidates(src, masked, false, SentenceWordCap) {
			head := src[:c.left]
			right, opened := openRest(src, masked, whole, c)
			h2, r2, o2 := carrierDivision(src, whole, c)
			seam := seamBefore(src, c.left)
			fmt.Fprintf(&b, "cut %d|%q aside=%v obj=%v open=%q/%d carrier=%q/%q/%d closesWhole=%v closesPhrase=%v\n", c.score, head[max(0, len(head)-20):], cutsAside(masked, c.left, c.right), splitsObject(whole, c), right, opened, h2[max(0, len(h2)-15):], r2, o2, closesWhole(head, seam, whole), closesPhrase(head, whole))
		}
	}
	t.Fatal(b.String())
}
