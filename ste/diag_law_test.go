package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestDiagPairLaw(t *testing.T) {
	var b strings.Builder
	for _, in := range []string{
		"The ONE pair-capacity law (pre-size envelope + adaptive high-water + budget/device clamps) — prepare() and growPairs() MUST agree byte-for- byte or growth would ping-pong through prepare()'s pairsChanged check.",
		"The ONE pair-capacity law (pre-size envelope + adaptive high-water + budget/device clamps) — prepare() and growPairs() MUST agree byte-for-byte or growth would ping-pong through prepare()'s pairsChanged check.",
	} {
		masked := checkMask(in)
		whole := syntax.Parse(masked, nil)
		fmt.Fprintf(&b, "\nIN: %s\nMASK: %s\nTAGS: %s\nOUTLINE:\n%s\n", in, masked, whole.Tags(), whole.Outline())
		next, divided := divideNext(in, SentenceWordCap)
		fmt.Fprintf(&b, "DIVIDE %v: %s\n", divided, next)
		s := syntax.Parse(mask(in), opaque(in, mask(in)))
		for _, d := range divisions(s, in) {
			fmt.Fprintf(&b, "DIV %+v admissible=%v left=%q rest=%q\n", d, admissible(s, in, d), in[:d.leftEnd], restOf(in, d))
		}
		for _, c := range candidates(in, masked, false, SentenceWordCap) {
			head, rest, _ := carrierOrNone(in, whole, c)
			r, opened := openRest(in, masked, whole, c)
			fmt.Fprintf(&b, "CUT %q | carrier %q | open %q %d | aside %v obj %v\n", in[:c.left], rest, r, opened, cutsAside(masked, c.left, c.right), splitsObject(whole, c))
			_ = head
		}
		hard, hok := hardDivision(in, masked, SentenceWordCap)
		forced, ok := forceDivision(in, masked, capSpec{cap: SentenceWordCap})
		fmt.Fprintf(&b, "HARD %v: %s\nFORCE %v: %s\n", hok, hard, ok, forced)
	}
	t.Error(b.String())
}
