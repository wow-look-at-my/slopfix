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

// A part never ends on a possessive, a stop is never doubled, and a name in
// lower case keeps its spelling.
func TestAForcedDivisionKeepsNamesAndStops(t *testing.T) {
	in := "The meter reads the headers of every upstream answer on the hot path of the proxy for the team and the fleet on the farm today because of requests. requireAuth then reads the bearer and the identity of the caller from the header of the request for the whole fleet."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.NotContains(t, got, "..", got)
	assert.Contains(t, got, "requireAuth", got)
	assert.NotContains(t, got, "RequireAuth", got)

	in = "The tool keeps every answer the hot path of the proxy reads for the whole team and the fleet across the farm in the mirror's cache of rows on the disk."
	got = ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.NotContains(t, got, "mirror's.", got)
}

// Bold that opens on a digit after a stop is an opener, not a closer.
func TestBoldOpeningOnADigitIsPaired(t *testing.T) {
	in := "Shape guard: `per_page` (1..100, default 30) and `page` (1..**40**, default 1 — the files API stops at 3000 files = 30 pages at the consumers' `per_page=100`, plus margin for the trailing empty page and smaller per_page shapes. Raised from 10."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
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
	assert.Empty(t, sentenceLengths(got), got)
	assert.NotContains(t, got, "detached.", got)
	assert.Contains(t, got, "a detached fetch", got)
}

// dash is the em dash dats writes between an aside and its sentence.
var dash = string(rune(0x2014))

// The paragraphs of dats docs/cli.md that the forced division wrote as "This
// is a command that genuinely needs the host is not a sandboxed command.",
// "This is from there", "This is use `inputs.copy`" and "This is not `/var`".
// "This is" goes only in front of a noun phrase or "for every run", with no verb of its own.
func TestAForcedDivisionNeverWritesThisIsInFrontOfAClauseOrAPhrase(t *testing.T) {
	for _, in := range []string{
		"**Writes** are confined to the file's temp directory (plus `--coverdir`, whose data has to outlive the run). There is deliberately no way to declare additional writable HOST paths: something to write is the temp directory " + dash + " a real filesystem inside every backend " + dash + " and a command that genuinely needs the host is not a sandboxed command, so it belongs to a `--no-sandbox` run. That includes a binary that rewrites itself on first run, such as an APE: copy it into the temp directory and run it from there, or run the file unsandboxed. To pull an *existing* host file into the temp directory so a command can modify a copy of it, use `inputs.copy` or `shared.copy` (see [file-format.md](file-format.md#copy-fixtures-inputscopy-and-sharedcopy)) " + dash + " the read-write counterpart of the working directory's read-only bind mount, resolved and copied before the sandbox starts.",
		"**Reads are confined under bwrap and docker**: a command sees the OS tool tree, the working directory, and the paths the file declared " + dash + " not `$HOME`, not `/var`, not another checkout on the machine. bwrap used to bind `/` read-only, which made every suite a reader of the whole host and made the two backends expose entirely different filesystems.",
	} {
		got := ste.Fix(in)
		for _, bad := range []string{"This is a command", "This is from there", "This is use", "This is not", "This is or"} {
			assert.NotContains(t, got, bad, got)
		}
	}
}

// A division never halves an aside between a pair of dashes, and never cuts a
// noun from the clause that describes it. These are dats docs/cli.md paragraphs
// a division wrote as "the temp directory. A real filesystem", "hooks. Runs at
// low OS priority" and "the paths. The file declared".
func TestADivisionKeepsADashAsideAndAReducedRelative(t *testing.T) {
	writes := "There is deliberately no way to declare additional writable HOST paths: something to write is the temp directory " + dash + " a real filesystem inside every backend " + dash + " and a command that genuinely needs the host is not a sandboxed command, so it belongs to a `--no-sandbox` run."
	nice := "Every spawned workload command " + dash + " test instances and setup/teardown hooks " + dash + " runs at low OS priority (nice 19 applied to the command's process group) so a heavily parallel run does not starve the machine."
	reads := "**Reads are confined under bwrap and docker**: a command sees the OS tool tree, the working directory, and the paths the file declared " + dash + " not `$HOME`, not `/var`, not another checkout on the machine."
	for in, bad := range map[string]string{
		writes: "directory. A real filesystem",
		nice:   "hooks. Runs",
		reads:  "the paths. The file declared",
	} {
		got := ste.Fix(in)
		assert.NotContains(t, got, bad, got)
	}
}

// The opening of unreal-tools namescrub/README.md, which a division wrote as
// "between cook. This is staging". A list item opens no sentence of its own.
func TestAForcedDivisionNeverOpensOnAListItem(t *testing.T) {
	in := "Redacts sensitive names (material parameter names, asset/package names, and the paths built from them) in Unreal Engine cooked output by **same-byte-length hash replacement** -- either on the loose cooked files between cook and staging, or directly **inside an existing classic `.pak`** (see Pak mode below)."
	got := ste.Fix(in)
	assert.NotContains(t, got, "This is staging", got)
	assert.NotContains(t, got, "between cook.", got)
}

// A division never ends a part on a verb and opens the rest on its object.
func TestAForcedDivisionKeepsAVerbWithItsObject(t *testing.T) {
	in := "A reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a careful repair rather than a quiet deletion."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
	assert.NotContains(t, got, "deserves.", got)
}

// A verb that opens the rest gets the subject again.
func TestAForcedDivisionRepeatsTheSubjectForAVerb(t *testing.T) {
	in := "The cache keeps every answer the upstream sent for the whole day across the restart of the process and the reload of the spec and holds the rows."
	got := ste.Fix(in)
	assert.Empty(t, sentenceLengths(got), got)
}
