package ste

import (
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestZZProbe(t *testing.T) {
	in := "The first word of a sentence names an item only in lower case or before a finite verb, because a capital there opens an instruction: \"Run 3 tests\"."
	m := checkMask(in)
	s := syntax.Parse(m, nil)
	var tags []string
	for _, w := range s.Words {
		tags = append(tags, w.Text+"/"+w.Tag)
	}
	t.Errorf("PROBE tags=%s", strings.Join(tags, " "))
	d, ok := divideNext(in, SentenceWordCap)
	t.Errorf("PROBE divideNext=%q %v", d, ok)
	t.Errorf("PROBE force=%q", forceSentenceCap(in, capSpec{cap: SentenceWordCap, reorder: true}))
	t.Errorf("PROBE splices=%q", fixSplices(in))
	t.Errorf("PROBE semis=%q", fixSemicolons(in))
}
