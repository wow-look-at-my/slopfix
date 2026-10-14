package ste

import (
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestDiagSplat2Tags(t *testing.T) {
	inputs := []string{
		"Measured fact: each real packs into the `8.33 ms` present windows at about `66%` duty, so one real spans about `4` windows (`33 ms`) against the `27.8 ms` content tick.",
		"Wave paths default ON wherever the ops exist.",
		"Move the locals into the class: locals computed in setup become plain members plus an assignment.",
		"Locals computed in setup become plain members plus an assignment.",
		"Each run must exit 0, report the registry slots and the stripe mode, report 0 validation errors and write its PNG.",
		"A missing validation layer fails the run, never skips.",
		"The run passed, and 17 gates sat \"Skipped\" in a green suite.",
	}
	var b strings.Builder
	for _, in := range inputs {
		s := syntax.Parse(checkMask(in), nil)
		b.WriteString("\nIN: " + in + "\nTAGS: " + s.Tags() + "\nOUTLINE:\n" + s.Outline() + "\n")
	}
	bullet := inputs[0]
	masked := checkMask(bullet)
	next, divided := divideNext(bullet, SentenceWordCap)
	forced, ok := forceDivision(bullet, masked, capSpec{cap: SentenceWordCap})
	hard, hok := hardDivision(bullet, masked, SentenceWordCap)
	b.WriteString("\nDIVIDE: " + next + " " + boolText(divided) + "\nFORCE: " + forced + " " + boolText(ok) + "\nHARD: " + hard + " " + boolText(hok) + "\n")
	b.WriteString("FIXCAP: " + fixSentenceCap(bullet, capSpec{cap: SentenceWordCap}) + "\n")
	t.Error(b.String())
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
