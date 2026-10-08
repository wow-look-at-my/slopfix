package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// A sentence that opens on a bare noun phrase, holds a dash aside. It ends on
// a parenthesis divides until every sentence is under the cap. The dashes are
// where it divides.
func TestADashAsideSentenceDividesUnderTheCap(t *testing.T) {
	in := "Only the `/// <reference lib=\"...\" />` closure of the libs the action can select -- lib.es2022.d.ts always, and lib.dom.d.ts + lib.dom.iterable.d.ts when a step passes dom: true -- seeded with any lib the staged type packages reference (@types/node pulls in a couple of esnext.* libs)."
	out := Fix(in)
	for _, s := range Sentences(checkMask(out)) {
		assert.LessOrEqual(t, WordCount(s), SentenceWordCap, "%s\n%s", out, traceForce(in))
	}
}

// traceForce writes what each forced division answers for source.
func traceForce(source string) string {
	masked := checkMask(source)
	whole := syntax.Parse(masked, nil)
	var b strings.Builder
	fmt.Fprintf(&b, "tags: %s\noutline: %s\n", whole.Tags(), whole.Outline())
	d := capSpec{cap: SentenceWordCap, reorder: true}
	for name, f := range map[string]func() (string, bool){
		"force":       func() (string, bool) { return forceDivision(source, masked, d) },
		"fragment":    func() (string, bool) { return fragmentDivision(source, masked, whole, d.cap) },
		"mark":        func() (string, bool) { return markDivision(source, masked, whole, d.cap) },
		"punctuation": func() (string, bool) { return punctuationDivision(source, masked, whole, d.cap) },
		"hard":        func() (string, bool) { return hardDivision(source, masked, d.cap) },
	} {
		out, ok := f()
		fmt.Fprintf(&b, "%s: ok=%v %q\n", name, ok, out)
	}
	fmt.Fprintf(&b, "cuts: %v asides: %v\n", punctuationCuts(masked, d.cap), asides(masked))
	for _, c := range candidates(source, masked, false, d.cap) {
		fmt.Fprintf(&b, "candidate left=%d right=%d words=%d seam=%q aside=%v head=%q\n", c.left, c.right, c.words, seamBefore(source, c.left), cutsAside(masked, c.left, c.right), source[:c.left])
	}
	return b.String()
}
