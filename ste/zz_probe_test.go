package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestZZProbe(t *testing.T) {
	var b strings.Builder
	for _, in := range []string{
		"main() holds the \"gpu\" lock for every splat-vulkan run except --help and --dump-layouts, so only one splat-vulkan uses the GPU at a time.",
		"captureReal writes new snapshot or history images into the generation no pending warp binds and flips to it before the next real frame starts its capture pass.",
		"The ONE pair-capacity law (pre-size envelope + adaptive high-water + budget/device clamps) — prepare() and growPairs() MUST agree byte-for-byte or growth would ping-pong through prepare()'s pairsChanged check.",
	} {
		masked := checkMask(in)
		whole := syntax.Parse(masked, nil)
		fmt.Fprintf(&b, "\nIN: %s\nTAGS: %s\nOUTLINE:\n%s\n", in, whole.Tags(), whole.Outline())
		for _, strict := range []bool{true, false} {
			for _, c := range candidates(in, masked, strict, SentenceWordCap) {
				head := in[:c.left]
				right, opened := openRest(in, masked, whole, c)
				seam := seamBefore(in, c.left)
				fmt.Fprintf(&b, "strict=%v CUT %q | open %q %d | seam %q cutsAside=%v splitsObject=%v closesWhole=%v closesPhrase=%v impMain=%v instructs=%v lists=%v\n",
					strict, head, right, opened, seam, cutsAside(masked, c.left, c.right), splitsObject(whole, c),
					closesWhole(head, seam, whole), closesPhrase(head, whole), opensImperativeMain(masked[:c.left]), instructs(whole), listsVerbs(masked[:c.left]))
			}
		}
		out, ok := forceDivision(in, masked, capSpec{cap: SentenceWordCap})
		fmt.Fprintf(&b, "FORCE %v: %s\n", ok, out)
	}
	t.Error(b.String())
}
