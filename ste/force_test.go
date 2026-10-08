package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// A sentence with no verb is a noun phrase. A phrase of place that describes a
// noun in it moves into a sentence of its own, behind that noun.
func TestALongSentenceWithNoVerbRestatesANoun(t *testing.T) {
	in := "The quick brown fox with the long red tail and the tiny black paws near the old wooden barn behind the tall green hills of the northern valley beside the cold river under the grey winter sky."
	require.NotEmpty(t, sentenceLengths(in), "the control: the sentence is over the cap")
	assert.Equal(t, "The quick brown fox with the long red tail and the tiny black paws near the old wooden barn. That barn is behind the tall green hills of the northern valley beside the cold river under the grey winter sky.", ste.Fix(in))
	assert.Empty(t, sentenceLengths(ste.Fix(in)))
}

// A division never lands inside a code span, a quotation, a parenthesis or bold text.
func TestAForcedDivisionKeepsEachSpanWhole(t *testing.T) {
	spans := []string{"`a code span with several words in it`", "\"a quoted phrase with several words\"", "(an aside with several words)", "**bold words here**"}
	in := "The long list of the old and new things near the " + spans[0] + " and the " + spans[1] + " and the " + spans[2] + " and the " + spans[3] + " for the whole team over the year."
	got := ste.Fix(in)
	for _, span := range spans {
		assert.Contains(t, got, span, got)
	}
}

// A sentence that opens on a code span keeps the span's first backtick. Without
// it the spans pair up wrong, and the division ends a part on "a".
func TestAForcedDivisionKeepsAnOpeningCodeSpan(t *testing.T) {
	in := "`<field>` declares a stored column: a name, a SQL-ish `type`, and a body that is a path into the absorbed document (`owner.login`) or a template. `store=\"document\"` on the resource stores the body."
	got := ste.Fix(in)
	assert.True(t, strings.HasPrefix(got, "`<field>` declares"), got)
	assert.NotContains(t, got, " or a.", got)
}

// A quote mark inside a code span pairs with nothing, as Check reads it. Paired
// with a later mark, it hid a whole paragraph from the division.
func TestAQuoteMarkInsideCodeOpensNoQuotation(t *testing.T) {
	in := "The tool reads `a\"b` and then the long list of the old and new things near the shed behind the barn across the field and the river for the team over the year with \"a quoted phrase\" here."
	got := ste.Fix(in)
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

// A part never ends on a possessive, a stop is never doubled, and a name in
// lower case keeps its spelling.
func TestAForcedDivisionKeepsNamesAndStops(t *testing.T) {
	in := "The meter reads the headers of every upstream answer on the hot path of the proxy for the team and the fleet on the farm today because of requests. requireAuth then reads the bearer and the identity of the caller from the header of the request for the whole fleet."
	got := ste.Fix(in)
	assert.NotContains(t, got, "..", got)
	assert.Contains(t, got, "requireAuth", got)
	assert.NotContains(t, got, "RequireAuth", got)

	in = "The tool keeps every answer the hot path of the proxy reads for the whole team and the fleet across the farm in the mirror's cache of rows on the disk."
	got = ste.Fix(in)
	assert.NotContains(t, got, "mirror's.", got)
}

// Bold that opens on a digit after a stop is an opener, not a closer.
func TestBoldOpeningOnADigitIsPaired(t *testing.T) {
	in := "Shape guard: `per_page` (1..100, default 30) and `page` (1..**40**, default 1 — the files API stops at 3000 files = 30 pages at the consumers' `per_page=100`, plus margin for the trailing empty page and smaller per_page shapes. Raised from 10."
	got := ste.Fix(in)
	assert.Contains(t, got, "**40**")
}

// The numeral repair reads the paragraph as Check does, so a long paragraph
// with code spans loses every numeral Check reports.
func TestAPostdeterminerInALongParagraphIsCut(t *testing.T) {
	in := "- **OAuth relay (github.com login endpoints)** — `POST /login/oauth/access_token` and `POST /login/device/code` relay the two browser-blocked `github.com` login endpoints, which are not on `api.github.com`. They share one core, `relayGitHubLogin` in `internal/api/oauth.go`. The pair covers the OAuth code-for-token exchange and the device-authorization start. Its body is opaque bytes to the relay."
	got := ste.Fix(in)
	for _, f := range ste.Check(got, 1) {
		assert.NotEqual(t, ste.IDPostdeterminer, f.ID, got)
	}
	assert.Contains(t, got, "relay the browser-blocked `github.com` login endpoints")
}

// The numeral repair finds what Check reports after a code span with a
// possessive and before a quotation.
func TestAPostdeterminerAfterCodeAndAQuotationIsCut(t *testing.T) {
	in := "A sweep that reached **no** repository at all is returned as an **error**, not as an empty result. Zero hits out of zero repositories says nothing about the pattern. And `grep`'s whole contract is that no matches means the text is not there. Letting this one case answer \"no matches\" will break exactly the guarantee the rest of the tool is built to keep."
	got := ste.Fix(in)
	for _, f := range ste.Check(got, 1) {
		assert.NotEqual(t, ste.IDPostdeterminer, f.ID, got)
	}
	assert.Contains(t, got, "this case", got)
}

// A forced division never lands inside a noun phrase, so "a detached fetch" stays whole.
func TestAForcedDivisionKeepsANounPhraseWhole(t *testing.T) {
	in := "- **A periodic sweep**, a **passthrough debounce** that shares one upstream call between identical concurrent reads (never across credentials), and **liveness paths** that hold while a detached fetch is in flight."
	got := ste.Fix(in)
	assert.NotContains(t, got, "detached.", got)
	assert.Contains(t, got, "a detached fetch", got)
}

// A verb that opens the rest gets the subject again.
func TestAForcedDivisionRepeatsTheSubjectForAVerb(t *testing.T) {
	in := "The cache keeps every answer the upstream sent for the whole day across the restart of the process and the reload of the spec and holds the rows."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
}

// A sentence that no clause boundary divides is still repaired: the fallback
// cuts between words, and Check reports nothing after it.
func TestALongSentenceWithNoDivisionIsRepaired(t *testing.T) {
	in := "gotext comes from a checkout for the reason stringer does, and for any of its own: completing x/text runs that module's cmd/gotext/examples/extract directive, which starts gotext -- the program this install exists to produce."
	require.NotEmpty(t, sentenceLengths(in), "the control: the sentence is over the cap")
	for _, f := range ste.Check(in, 1) {
		assert.False(t, ste.ByHand(f.Fix), "no finding asks for a rewrite by hand: %s", f.Fix)
	}
	fixed := ste.Fix(in)
	assert.NotEqual(t, in, fixed)
	assert.Empty(t, sentenceLengths(fixed), fixed)
	assert.Equal(t, fixed, ste.Fix(fixed), "the same sentence writes the same repair")
	for _, s := range ste.Sentences(fixed) {
		assert.LessOrEqual(t, ste.WordCount(s), ste.SentenceWordCap, fixed)
	}
}

// The fallback is bounded: a sentence already under the cap is left as it is,
// and a one-word sentence is never cut.
func TestAShortSentenceIsLeftAlone(t *testing.T) {
	for _, in := range []string{"One.", "Two words.", "The cache holds every answer from the upstream for the whole day."} {
		assert.Empty(t, sentenceLengths(in), in)
		assert.Equal(t, in, ste.Fix(in), in)
	}
}
