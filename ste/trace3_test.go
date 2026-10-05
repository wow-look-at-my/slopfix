package ste

import (
	"fmt"
	"os"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestTraceAndThat(t *testing.T) {
	in := "The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs."
	masked := mask(in)
	whole := syntax.Parse(masked, nil)
	out := ""
	for _, c := range candidates(in, masked, false, SentenceWordCap) {
		rest := trim(in[c.right:])
		first := wordFrom(whole, len(in)-len(rest))
		if first < 1 {
			continue
		}
		head, carried, score := carrierDivision(in, whole, c)
		out += fmt.Sprintf("cut@%d seam=%q first=%d word=%s/%s headOpen=%v hasMain=%v carrier_head=%q carried=%q score=%d\n",
			c.left, seamBefore(in, c.left), first, whole.Words[first].Lower(), whole.Words[first].Tag, headOpen(whole, first), hasMain(whole, c.left), head, carried, score)
	}
	f, ok := forceDivision(in, masked, capSpec{reorder: true, cap: SentenceWordCap})
	out += fmt.Sprintf("forceDivision ok=%v out=%q\n", ok, f)
	_ = os.WriteFile("../.scratch/trace3.txt", []byte(out), 0o644)
}

func hasMain(s *syntax.Sentence, at int) bool {
	_, ok := mainVerb(s, at)
	return ok
}

func trim(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}
