package ste_test

import (
	"fmt"
	"os"
	"sort"
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
// the cap, or a phrase the source already used. That the repair wrote again.
func capReport(in, out string) string {
	var over, dup []string
	for _, s := range ste.Sentences(ste.Masked(out)) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if n := ste.WordCount(s); n > ste.SentenceWordCap {
			over = append(over, fmt.Sprintf("%d=%s", n, s))
		}
	}
	were := map[string]int{}
	inw := words(in)
	for i := 0; i+3 <= len(inw); i++ {
		were[strings.Join(inw[i:i+3], " ")]++
	}
	now := map[string]int{}
	outw := words(out)
	for i := 0; i+3 <= len(outw); i++ {
		now[strings.Join(outw[i:i+3], " ")]++
	}
	for gram, n := range now {
		if were[gram] > 0 && n > were[gram] {
			dup = append(dup, gram)
		}
	}
	sort.Strings(dup)
	if len(over) == 0 && len(dup) == 0 {
		return ""
	}
	return fmt.Sprintf("IN: %s\nOUT: %s\nOVER: %s\nREPEATED: %s\n\n",
		in, out, strings.Join(over, " | "), strings.Join(dup, " | "))
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
