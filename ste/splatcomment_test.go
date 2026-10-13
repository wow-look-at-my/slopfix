package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// Comment sentences from splat-vulkan that the comment repair garbled. Each
// divides into grammatical sentences, and a failure traces every division the
// repair weighed.
var splatComments = []struct{ in, want string }{
	{
		"Pair demand at quality q relative to now: q split over the free rungs the way applyQuality spends it (before the res lattice), each rung's ratio raised to its measured elasticity.",
		"Pair demand at quality q relative to now. This is q split over the free rungs the way applyQuality spends it (before the res lattice), each rung's ratio raised to its measured elasticity.",
	},
	{
		"animation-prepack-equivalence - the animation pre-pack's byte gate: every adopted frame of a synthetic clip lands records, cubeBuf, C and a rendered image equal to the live async tick wire's, on the copy and the in-place (UMA direct) row wires; a repeated C reuses its pipeline triple; an edge repack of an adopted frame and a resumed live tick match too.",
		"animation-prepack-equivalence - the animation pre-pack's byte gate. Every adopted frame of a synthetic clip lands records and cubeBuf. Every adopted frame also lands C and a rendered image equal to the live async tick wire's. That frame is on the copy and the in-place (UMA direct) row wires. A repeated C reuses its pipeline triple. An edge repack of an adopted frame and a resumed live tick match too.",
	},
	{
		"It carries a capital, a digit, an underscore or a dot inside it, or it is a single letter other than the article, as a variable such as q is.",
		"It carries a capital, a digit, an underscore or a dot inside it. Otherwise, it is a single letter other than the article, as a variable such as q is.",
	},
	{
		"Per octant, total mass W = sum o_i sqrt(det Sigma_i), mean mu, second moment Sigma_m, mass-weighted color, merged opacity o_m = min(1, W / sqrt(det Sigma_m)), and per cube the luma variance.",
		"Per octant, total mass W = sum o_i sqrt(det Sigma_i), mean mu, second moment Sigma_m, mass-weighted color. This also covers merged opacity o_m = min(1, W / sqrt(det Sigma_m)), and per cube the luma variance.",
	},
}

// A list after a colon that a clause opens, behind a conjunction, divides under
// the cap with no carrier in front of the conjunction.
func TestAConjoinedClauseBeforeAColonListDividesUnderTheCap(t *testing.T) {
	dash := string(rune(0x2014))
	in := "Then `(deny file-read*)` and `(deny file-write*)` take every file access away, and the rules that follow give each kind of access back only for the paths the jail is supposed to expose: - `/dev` (a terminal, a PTY, `/dev/null`) " + dash + " read and write. - a `--rw` mount " + dash + " read and write."
	out := Fix(in)
	assert.NotContains(t, out, "This is and", "%s", traceStages(in))
	for _, s := range Sentences(checkMask(out)) {
		assert.LessOrEqual(t, WordCount(s), SentenceWordCap, "%s\n%s", out, traceStages(in))
	}
}

func TestSplatCommentSentencesDivideGrammatically(t *testing.T) {
	divides := func(id string) bool { return id == IDSentenceCap || id == IDSemicolon || id == IDCommaSplice }
	for _, c := range splatComments {
		got := FixSelected(c.in, divides)
		assert.Equal(t, c.want, got, "%s", traceStages(c.in))
	}
}

// traceStages writes the semicolon repair, then the parse, the clause
// divisions and the forced divisions of every sentence over the cap after it.
func traceStages(in string) string {
	var b strings.Builder
	semi := fixSemicolons(in)
	fmt.Fprintf(&b, "semicolons: %s\n", semi)
	for _, sentence := range Sentences(semi) {
		if WordCount(checkMask(sentence)) <= SentenceWordCap {
			continue
		}
		masked := mask(sentence)
		s := syntax.Parse(masked, opaque(sentence, masked))
		fmt.Fprintf(&b, "== %s\ntags: %s\noutline: %s\n%s%s", sentence, s.Tags(), s.Outline(), traceDivisions(s, sentence), traceForce(sentence))
	}
	return b.String()
}
