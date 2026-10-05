package commentfix

import (
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestZZProbe(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("a clause that never closes and keeps going onward ", 3))
	h, ok := ste.PhraseHead(text, 25)
	t.Errorf("PROBE head=%q ok=%v", h, ok)
	s := syntax.Parse(text, nil)
	var tags []string
	for _, w := range s.Words {
		tags = append(tags, w.Text+"/"+w.Tag)
	}
	t.Errorf("PROBE tags=%v", tags)
	body := strings.Repeat("// a clause that never closes and just keeps going onward\n", 8)
	src := "package p\n\n" + body + "const p = 1\n"
	for _, b := range blocks("x.go", src) {
		fit := func(sentence, indent, marker string) ([]string, bool) {
			out := reflow(sentence, indent, marker, wrapWidth)
			return out, fitsCode(out, b)
		}
		o1, w1 := steOpening(b.text, fit, false)
		o2, w2 := steOpening(b.text, fit, true)
		t.Errorf("PROBE open=%q %v shrink=%q %v repair=%q", o1, w1, o2, w2, repair(b))
	}
}
