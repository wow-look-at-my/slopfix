package ste_test

import (
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/ste"
)

func TestZZProbe(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("a clause that never closes and keeps going onward ", 8)) + "."
	t.Errorf("PROBE Fix=%q", ste.Fix(text))
	t.Errorf("PROBE Keep=%q", ste.FixKeepingOpening(text))
	t.Errorf("PROBE Div15=%q", ste.DivideTo(text, 15))
}
