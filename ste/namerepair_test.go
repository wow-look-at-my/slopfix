package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// only keeps the rule a case drives.
func only(id string) func(string) bool {
	return func(got string) bool { return got == id }
}

// A dictionary word inside a path is part of a name another file resolves.
func TestDictionaryLeavesAPathAlone(t *testing.T) {
	text := "See docs/reveal-layer.md for principal resolution."
	assert.Equal(t, text, ste.FixSelected(text, only(ste.IDDictionary)))
	assert.Empty(t, warned(ste.IDDictionary, text))
}

// Each joiner of a name keeps the word in it as written.
func TestDictionaryLeavesEveryNameAlone(t *testing.T) {
	for _, text := range []string{
		"Read reveal.go before the edit.",
		"Call api.reveal for the row.",
		"Set reveal_mode in the file.",
		"Open https://example.com/reveal now.",
		"Call ns::reveal for the row.",
		"Pass --reveal to the tool.",
		"Read reveal2 before the edit.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, only(ste.IDDictionary)), text)
		assert.Empty(t, warned(ste.IDDictionary, text), text)
	}
}

// The control. A dictionary word in prose still takes its approved word, a
// hyphenated compound and a word before a period included.
func TestDictionaryStillRewritesProse(t *testing.T) {
	cases := map[string]string{
		"The reveal layer gates every read.":    "The show layer gates every read.",
		"It does not reveal-check the row.":     "It does not show-check the row.",
		"The gate does not reveal.":             "The gate does not show.",
		"Reveal the row: the gate reads it.":    "Show the row: the gate reads it.",
		"Read docs/a.md to reveal the row.":     "Read docs/a.md to show the row.",
	}
	for in, want := range cases {
		assert.Equal(t, want, ste.FixSelected(in, only(ste.IDDictionary)), in)
	}
	assert.NotEmpty(t, warned(ste.IDDictionary, "The reveal layer gates every read."))
}

// warned answers the findings of one warning rule in text.
func warned(id, text string) []ste.Finding {
	var out []ste.Finding
	for _, f := range ste.Warn(text, 1, false) {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}
