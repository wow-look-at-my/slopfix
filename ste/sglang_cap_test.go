package ste_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// words lists the alphabetic words of text in lower case.
func words(text string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && r != '\''
	}) {
		out = append(out, f)
	}
	return out
}

// capReport names what a repair of one sentence got wrong: a part still over
// the cap, a part. That is not a sentence of its own, or a word it repeated.
func capReport(in, out string) string {
	inw := map[string]int{}
	for _, w := range words(in) {
		inw[w]++
	}
	outw := map[string]int{}
	var over, frag, dup []string
	for _, s := range ste.Sentences(ste.Masked(out)) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if n := ste.WordCount(s); n > ste.SentenceWordCap {
			over = append(over, fmt.Sprintf("%d=%s", n, s))
		}
		if !ste.StandsAlone(s) {
			frag = append(frag, s)
		}
	}
	for _, w := range words(out) {
		outw[w]++
		if outw[w] > inw[w] {
			dup = append(dup, w)
		}
	}
	if len(over) == 0 && len(frag) == 0 && len(dup) == 0 {
		return ""
	}
	return fmt.Sprintf("IN: %s\nOUT: %s\nOVER: %s\nFRAGMENT: %s\nREPEATED: %s\n\n",
		in, out, strings.Join(over, " | "), strings.Join(frag, " | "), strings.Join(dup, " "))
}

func TestEveryExtractedSentenceDividesUnderTheCap(t *testing.T) {
	var report strings.Builder
	failed := 0
	for _, in := range capSentences {
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
	require := os.WriteFile("../.scratch/cap-report.txt", []byte(report.String()), 0o644)
	assert.NoError(t, require)
	t.Logf("sentences the repair got wrong: %d of the fixture", failed)
	assert.Zero(t, failed, "see .scratch/cap-report.txt")
}
