package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestDiagOrCarrier(t *testing.T) {
	in := "The rule answers the number at offset at and the words up to the plural noun it counts, as in \"0 validation errors\", or \"\" when no plural noun ends a phrase it governs."
	masked := checkMask(in)
	whole := syntax.Parse(masked, nil)
	var b strings.Builder
	b.WriteString("TAGS: " + whole.Tags() + "\nMASK: " + masked + "\n")
	main, hasMain := mainVerb(whole, len(in))
	fmt.Fprintf(&b, "MAIN %v %+v\n", hasMain, main)
	next, divided := divideNext(in, SentenceWordCap)
	fmt.Fprintf(&b, "DIVIDE %v: %s\n", divided, next)
	for _, c := range candidates(in, masked, false, SentenceWordCap) {
		head, rest, score := carrierOrNone(in, whole, c)
		r, opened := openRest(in, masked, whole, c)
		fmt.Fprintf(&b, "CUT %d/%d %q | %q | carrier %q %q %d | open %q %d\n", c.left, c.right, in[:c.left], in[c.right:], head, rest, score, r, opened)
	}
	forced, ok := forceDivision(in, masked, capSpec{cap: SentenceWordCap})
	fmt.Fprintf(&b, "FORCE %v: %s\n", ok, forced)
	t.Error(b.String())
}
