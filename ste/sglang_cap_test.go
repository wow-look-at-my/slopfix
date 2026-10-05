package ste_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// words lists the alphabetic words of text in lower case. A repair is
// checked for the words it keeps rather than the punctuation it moves.
func words(text string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && r != '\''
	}) {
		out = append(out, f)
	}
	return out
}

// capReport writes one line per sentence the repair leaves over the cap or
// shortens, so a run names exactly what still fails.
func capReport(in, out string) string {
	var over []string
	for _, s := range ste.Sentences(ste.Masked(out)) {
		if n := ste.WordCount(s); n > ste.SentenceWordCap {
			over = append(over, fmt.Sprintf("%d=%s", n, strings.TrimSpace(s)))
		}
	}
	got := map[string]int{}
	for _, w := range words(out) {
		got[w]++
	}
	var dropped []string
	for _, w := range words(in) {
		got[w]--
		if got[w] < 0 {
			dropped = append(dropped, w)
		}
	}
	if len(over) == 0 && len(dropped) == 0 {
		return ""
	}
	return fmt.Sprintf("IN: %s\nOUT: %s\nOVER: %s\nDROPPED: %s\n\n", in, out, strings.Join(over, " | "), strings.Join(dropped, " "))
}

func TestEveryExtractedSentenceDividesUnderTheCap(t *testing.T) {
	data, err := os.ReadFile("testdata/sglang-sentence-cap.txt")
	require.NoError(t, err)
	var report strings.Builder
	failed := 0
	for _, in := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		out := ste.Fix(in)
		if r := capReport(in, out); r != "" {
			failed++
			report.WriteString(r)
		}
	}
	require.NoError(t, os.WriteFile("../.scratch/cap-report.txt", []byte(report.String()), 0o644))
	t.Logf("sentences over cap or shortened: %d of the fixture", failed)
	assert.Zero(t, failed, "see testdata/cap-report.txt")
}
