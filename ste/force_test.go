package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// sentenceLengths answers the findings Check reports under the cap rule.
func sentenceLengths(text string) []ste.Finding {
	var out []ste.Finding
	for _, f := range ste.Check(text, 1) {
		if f.ID == ste.IDSentenceCap {
			out = append(out, f)
		}
	}
	return out
}

// A sentence with no clause boundary still divides, between words.
func TestALongSentenceWithNoBoundaryDividesBetweenWords(t *testing.T) {
	in := "The quick brown fox with the long red tail and the tiny black paws near the old wooden barn behind the tall green hills of the northern valley beside the cold river under the grey winter sky."
	assert.NotEmpty(t, sentenceLengths(in), "the control: the sentence is over the cap")
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.NotEqual(t, in, got)
	for _, word := range strings.Fields(strings.Trim(in, ".")) {
		assert.Contains(t, got, strings.Trim(word, "."), "no word is lost")
	}
}

// A forced division never lands inside a code span, a quotation, a parenthesis or bold text.
func TestAForcedDivisionKeepsEachSpanWhole(t *testing.T) {
	spans := []string{"`a code span with several words in it`", "\"a quoted phrase with several words\"", "(an aside with several words)", "**bold words here**"}
	in := "The long list of the old and new things near the " + spans[0] + " and the " + spans[1] + " and the " + spans[2] + " and the " + spans[3] + " for the whole team over the year."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	for _, span := range spans {
		assert.Contains(t, got, span, got)
	}
}

// A sentence that opens on a code span keeps the span's first backtick. Without
// it the spans pair up wrong, and the division ends a part on "a".
func TestAForcedDivisionKeepsAnOpeningCodeSpan(t *testing.T) {
	in := "`<field>` declares a stored column: a name, a SQL-ish `type`, and a body that is a path into the absorbed document (`owner.login`) or a template. `store=\"document\"` on the resource stores the body."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.True(t, strings.HasPrefix(got, "`<field>` declares"), got)
	assert.NotContains(t, got, " or a.", got)
}

// A quote mark inside a code span pairs with nothing, as Check reads it. Paired
// with a later mark, it hid a whole paragraph from the division.
func TestAQuoteMarkInsideCodeOpensNoQuotation(t *testing.T) {
	in := "The tool reads `a\"b` and then the long list of the old and new things near the shed behind the barn across the field and the river for the team over the year with \"a quoted phrase\" here."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.Contains(t, got, "`a\"b`")
	assert.Contains(t, got, "\"a quoted phrase\"")
}

// A division never drops half of a bold pair, and every sentence keeps its pairs.
func TestADivisionKeepsBoldPaired(t *testing.T) {
	in := "Its **`mergeable` is KNOWN** (MERGEABLE/CONFLICTING) **AND its `mergeable_state` is known and current** (non-empty, not `unknown`) — the state is part of the gate rather than a field that rides along with the rest of the row."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.Equal(t, 4, strings.Count(got, "**"), got)
	for _, sentence := range ste.Sentences(got) {
		assert.Equal(t, 0, strings.Count(sentence, "**")%2, "%q leaves a bold pair open", sentence)
	}
}

// A verb that opens the rest gets the subject again.
func TestAForcedDivisionRepeatsTheSubjectForAVerb(t *testing.T) {
	in := "The cache keeps every answer the upstream sent for the whole day across the restart of the process and the reload of the spec and holds the rows."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
}
