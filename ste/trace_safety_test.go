package ste

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// TestTraceTheSafetyCut names the gate each candidate cut dies at, so the
// producer of "...that no other. Thread writes through it..." is named.
func TestTraceTheSafetyCut(t *testing.T) {
	in := "The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs."
	masked := mask(in)
	whole := syntax.Parse(masked, nil)
	var b strings.Builder
	for _, strict := range []bool{true, false} {
		for _, c := range candidates(in, masked, strict, SentenceWordCap) {
			head := in[:c.left]
			right, opened := openRest(in, masked, whole, c)
			if right == "" {
				head, right, opened = carrierDivision(in, whole, c)
			}
			left := closeHead(head)
			seam := seamBefore(in, c.left)
			if seam == "," && hasSoPrefix(in[c.right:]) {
				seam = "so"
			}
			frag := head == fragmentHead(in[:c.left])
			fmt.Fprintf(&b, "strict=%v cut@%d left=%q right=%q opened=%v cutsAside=%v splitsObject=%v frag=%v divides=%v closesWhole=%v closesPhrase=%v ACCEPT=%v\n",
				strict, c.left, left, right, opened,
				cutsAside(masked, c.left, c.right), splitsObject(whole, c),
				frag, right != "" && divides(left, right, SentenceWordCap),
				closesWhole(in[:c.left], seam, whole), closesPhrase(in[:c.left], whole),
				right != "" && divides(left, right, SentenceWordCap) && (frag || closesWhole(in[:c.left], seam, whole) || closesPhrase(in[:c.left], whole)))
		}
	}
	best, ok := bestDivision(syntax.Parse(masked, nil), in)
	fmt.Fprintf(&b, "\nbestDivision=%q ok=%v\n", best, ok)
	f, _ := forceDivision(in, masked, capSpec{reorder: true, cap: SentenceWordCap})
	fmt.Fprintf(&b, "forceDivision=%q\n", f)
	fmt.Fprintf(&b, "standsAsSentence(no other.)=%v\n", standsAsSentence("The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other."))
	tags := syntax.Parse(masked, nil)
	fmt.Fprintf(&b, "tags: %s\noutline: %s\n", tags.Tags(), tags.Outline())
	require.NoError(t, os.WriteFile("../.scratch/trace.txt", []byte(b.String()), 0o644))

}

func hasSoPrefix(s string) bool {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return len(s) >= 3 && s[:3] == "so "
}
