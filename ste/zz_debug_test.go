package ste

import (
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestZZDebugParse(t *testing.T) {
	text := strip("A downloaded analyzer needs its engine beside it (buildhost project `vega-analyzer/spv2gcn-engine`), and a downloaded vkbench needs `vkb-engine` (buildhost project `vega-analyzer/vkb-engine`).")
	s := syntax.Parse(text, opaque(text, text))
	t.Fatalf("\n%s\n%s\n%s\nspans=%v", s.Tags(), s.Brackets(), s.Outline(), sentenceSpans(text))
}
