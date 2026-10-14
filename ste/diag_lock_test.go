package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestDiagLock(t *testing.T) {
	var b strings.Builder
	for _, in := range []string{
		"Every request takes a ticket in \"<lock file>.queue/\": a sequence number from the counter file, taken under flock on that file, names a ticket file that its owner holds flock(LOCK_EX) on for as long as it waits or holds.",
		"main() holds the \"gpu\" lock for every splat-vulkan run except --help and --dump-layouts, so only one splat-vulkan uses the GPU at a time.",
	} {
		masked := checkMask(in)
		whole := syntax.Parse(masked, nil)
		fmt.Fprintf(&b, "\nIN: %s\nTAGS: %s\nOUTLINE:\n%s\n", in, whole.Tags(), whole.Outline())
		next, divided := divideNext(in, SentenceWordCap)
		fmt.Fprintf(&b, "DIVIDE %v: %s\n", divided, next)
		s := syntax.Parse(mask(in), opaque(in, mask(in)))
		for _, d := range divisions(s, in) {
			left := strings.TrimRight(in[:d.leftEnd], " ,;:") + "."
			right := joinOpener(d.opener, restOf(in, d))
			fmt.Fprintf(&b, "DIV adm=%v leftStands=%v rightStands=%v fin=%v left=%q right=%q\n", admissible(s, in, d), standsAsSentence(left), standsAsSentence(right),
				finiteOutsideRelative(syntax.Parse(checkMask(right), nil), 0), left, right)
		}
		for _, c := range candidates(in, masked, false, SentenceWordCap) {
			_, rest, _ := carrierOrNone(in, whole, c)
			r, opened := openRest(in, masked, whole, c)
			if rest == "" && r == "" {
				continue
			}
			fmt.Fprintf(&b, "CUT %q | carrier %q | open %q %d\n", in[:c.left], rest, r, opened)
		}
		forced, ok := forceDivision(in, masked, capSpec{cap: SentenceWordCap})
		fmt.Fprintf(&b, "FORCE %v: %s\nFIX: %s\n", ok, forced, fixSentenceCap(in, capSpec{cap: SentenceWordCap}))
	}
	t.Error(b.String())
}
