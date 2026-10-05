package counts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/ste"
)

func phrases(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Phrase)
	}
	return out
}

func TestEachFrameReportsItsCount(t *testing.T) {
	for _, doc := range []string{
		"This repo's 15 plugins ride in the payload.",
		"It ships two hooks.",
		"There are three sections.",
		"The four rules below decide it.",
	} {
		assert.NotEmpty(t, Check(doc), doc)
	}
}

func TestAQuantityWithoutAFrameIsLeftAlone(t *testing.T) {
	for _, doc := range []string{
		"Every plugin this repo installs rides in the payload.",
		"Split it into three parts if that reads better.",
		"Version 2.1.205 clients keep the builtin.",
	} {
		assert.Empty(t, Check(doc), doc)
	}
}

// A limit and a size rot exactly as a tally does. A budget gets raised and a
// suite gets slower, and the document keeps asserting the old value.
func TestAMeasurementIsACount(t *testing.T) {
	assert.NotEmpty(t, Check("The read has 20 seconds."))
	assert.NotEmpty(t, Check("It carries 500 lines."))
	// A doc recorded the range a build measured, and the range moved.
	assert.NotEmpty(t, Check("An unchanged second build measures 60 seconds."))
	assert.NotEmpty(t, Check("The step takes about 90 seconds."))
	assert.Empty(t, Check("The budget lives in ci.yml, which is where to read it."))
}

func TestAFunctionWordBreaksTheCount(t *testing.T) {
	assert.Empty(t, Check("it has 2 of the format drops"))
}

// The exemptions come from the markdown splitter, so a fence, a table, a
// heading and an indented block are all data.
func TestVerbatimBlocksAreNeverJudged(t *testing.T) {
	assert.Empty(t, Check("```go\n// it has three sections\n```"))
	assert.Empty(t, Check("| it has three sections |\n|---|"))
	assert.Empty(t, Check("# it has three sections"))
	assert.Empty(t, Check("    it has three sections"))
}

func TestAnInlineCodeSpanIsData(t *testing.T) {
	assert.Empty(t, Check("Write `there are three sections` instead."))
}

func TestStripCutsTheNumberAndKeepsTheRest(t *testing.T) {
	out, cut := Strip("This repo's 15 plugins ride in the payload.")
	assert.NotContains(t, out, "15")
	assert.Contains(t, out, "ride in the payload", "only the count went")
	assert.NotEmpty(t, phrases(cut))
}

// Back to front, so an earlier span's offsets stay valid.
func TestStripHandlesSeveralCountsInOneDocument(t *testing.T) {
	out, cut := Strip("It ships two hooks.\n\nThere are three sections.\n")
	require.Len(t, cut, 2)
	assert.NotContains(t, out, "two")
	assert.NotContains(t, out, "three")
	assert.Contains(t, out, "\n\n", "the paragraph break survived both cuts")
}

// A count that opens a sentence gives its capital to the next word. Each case
// is a sentence from the go-toolchain docs.
func TestStripKeepsTheCapitalOfASentenceItOpens(t *testing.T) {
	for in, want := range map[string]string{
		"Default `.` becomes `root`. Two builds in one job therefore save distinct hand-offs and can no longer 409 on a shared key.": "Default `.` becomes `root`. Builds in one job therefore save distinct hand-offs and can no longer 409 on a shared key.",
		"It came from the finished-test durations. Two attempts at intra-package parallelism failed and were reverted:":              "It came from the finished-test durations. Attempts at intra-package parallelism failed and were reverted:",
		"It is built. Two inputs vary between runners and each flag closes one.":                                                     "It is built. Inputs vary between runners and each flag closes one.",
		"It ships two hooks.": "It ships hooks.",
	} {
		out, cut := StripGate(in)
		assert.NotEmpty(t, cut, in)
		assert.Equal(t, want, out)
	}
}

func TestACountAfterANounBecomesMultiple(t *testing.T) {
	out, cut := StripGate("The Linux syscall takes no flags, and one APE must not answer one call two ways.")
	assert.Equal(t, "The Linux syscall takes no flags, and one APE must not answer one call multiple ways.", out, "a bare cut leaves \"one call ways\"")
	assert.NotEmpty(t, cut)

	out, _ = StripGate("against binaries built on all three platforms.")
	assert.Equal(t, "against binaries built on all platforms.", out, "a determiner before the count still lets it go")
}

// The lines of a real document that names its branches by number, and then
// cites them by number alone.
const attributionVerdict = "### 2. Keyless fallback (branch 3) ambiguity\n\n" +
	"- branch (1) the still-streaming `current_agent_msg`, guarded by `streaming_matches`.\n" +
	"- branch (2) the prompt→entry map `finished_prompt_costs` recorded by `finish_turn`.\n" +
	"- branch (3) the keyless fallback `last_finished_agent_entry`.\n\n" +
	"- during a newer stream → dropped (branch 1 prompt-mismatch rejects, branch 3 sees `last_finished_agent_entry` already cleared).\n"

// Digits after a singular noun name an item, and a point on a scale is a
// value. Each goes stale like a count, so each is a finding. The repair names
// the item with the words the document gives it, and a generic phrase where it
// gives none. "branch multiple sees" is no English, so no repair writes it.
func TestANumberThatNamesAnItemIsRepaired(t *testing.T) {
	out, cut := StripGate(attributionVerdict)
	assert.NotEmpty(t, cut)
	assert.Contains(t, out, "(the still-streaming `current_agent_msg` prompt-mismatch rejects, the keyless fallback sees `last_finished_agent_entry` already cleared).", out)
	assert.Contains(t, out, "- branch (3) the keyless fallback", "the definition stays as written")

	for in, want := range map[string]string{
		"Then branch 3 sees the entry already cleared.":    "Then a later branch sees the entry already cleared.",
		"They honor the Section 4 pins.":                   "They honor the pins of a later section.",
		"It reads rule 6 inputs.":                          "It reads the inputs of a later rule.",
		"The mock with id 1 completes after a short delay.": "The mock with one id completes after a short delay.",
		"The row wraps at 100 cols in the modal.":          "The row wraps at a set number of cols in the modal.",
		"The preview contributes 0 lines when it is off.":  "The preview contributes no lines when it is off.",
	} {
		var reported bool
		for _, f := range ste.Check(in, 1) {
			reported = reported || f.ID == ste.IDStaleCount
		}
		assert.True(t, reported, "check reports %q", in)
		out, cut := StripGate(in)
		assert.Equal(t, want, out)
		assert.NotEmpty(t, cut)
		for _, f := range ste.Check(out, 1) {
			assert.NotEqual(t, ste.IDStaleCount, f.ID, f.Detail)
		}
	}
}

// Every count the rules report gets a repair. A bare cut is not English in
// each of these, so the number gives way to words that claim no figure.
func TestEveryCountIsReworded(t *testing.T) {
	for in, want := range map[string]string{
		"It polls every 15 minutes.":            "It polls every few minutes.",
		"Each 3 builds it prunes the cache.":    "Every few builds it prunes the cache.",
		"It sends one request per 10 seconds.":  "It sends one request every few seconds.",
		"The step takes about 90 seconds.":      "The step takes many seconds.",
		"The read has 20 seconds.":              "The read has several seconds.",
		"It retries two times.":                 "It retries a couple of times.",
		"It keeps at most 500 lines.":           "It keeps a bounded number of lines.",
		"It keeps up to 10 entries.":            "It keeps a bounded number of entries.",
		"It needs at least 3 reviewers.":        "It needs a few reviewers.",
		"It carries over 500 lines.":            "It carries over many lines.",
		"It runs in two passes.":                "It runs in multiple passes.",
		"It ships two hooks.":                   "It ships hooks.",
		"It ships exactly two hooks.":           "It ships hooks.",
		"There are only three sections.":        "There are sections.",
		"It holds 12345 files.":                 "It holds files.",
		"Done. 15 plugins ride in the payload.": "Done. Plugins ride in the payload.",
		"Done. 30 seconds pass first.":          "Done. Many seconds pass first.",
		"It came from ~40 sources.":             "It came from multiple sources.",
		"Every 30 seconds it retries.":          "Every few seconds it retries.",
	} {
		out, cut := StripGate(in)
		assert.NotEmpty(t, cut, in)
		assert.Equal(t, want, out, in)
	}
}

// Each of these is the cut a document reader caught. The repair either keeps
// the sentence English, or leaves a number that counts nothing alone.
func TestARewordKeepsTheSentenceEnglish(t *testing.T) {
	for in, want := range map[string]string{
		"Vega10 has 16 RBs over 4 shader engines.":               "Vega10 has 16 RBs over a few shader engines.",
		"It drives 35 fixtures over six seeded inputs.":          "It drives fixtures over several seeded inputs.",
		"The integer form spreads over 32 banks.":                "The integer form spreads over many banks.",
		"One round trip amortized over 40 resident waves hides.": "One round trip amortized over many resident waves hides.",
		"glslang can emit a ternary over two samples.":           "glslang can emit a ternary over a couple of samples.",
		"It describes an engine carrying FOUR render backends.":  "It describes an engine carrying render backends.",
		"It is a pass over the same 262144 covered pixels.":      "It is a pass over the same covered pixels.",
		"It runs on the other three platforms.":                  "It runs on the other platforms.",
	} {
		out, cut := StripGate(in)
		assert.NotEmpty(t, cut, in)
		assert.Equal(t, want, out, in)
	}
}

// An issue number, a Vega model, a word size and an equation each name a fixed
// value. None counts the plural noun after it.
func TestAFixedValueIsNoCount(t *testing.T) {
	for _, in := range []string{
		"The corpus that issue #54 targets compiles.",
		"It reports verdicts for the analyzer Vega 11 constants.",
		"It widens the table pointer to 64 bits.",
		"The field holds 32 bits.",
		"The lane carries 16 bits.",
		"The mask keeps 8 bits.",
		"The footprint (1760 workgroups x 64 KiB = 110 MiB) fits.",
		"The §9 trigger fires.",
		"Is a fork as cheap to create as §2.4 claims?",
		"It inherits §3 rules without re-arguing them.",
	} {
		out, cut := StripGate(in)
		assert.Empty(t, cut, in)
		assert.Equal(t, in, out, in)
	}
}

// Only "issue #<digits>" is exempt. A list position goes stale when the list is
// renumbered, so it is a count.
func TestOnlyAnIssueNumberIsALabel(t *testing.T) {
	for _, in := range []string{
		"The parser keeps #12 rows.",
		"It merges pr #12 files.",
		"It is the loop that gate 5 asserts.",
		"The field holds 24 bits.",
	} {
		_, cut := StripGate(in)
		assert.NotEmpty(t, cut, in)
	}
}

// No count the rules report is left for a person.
func TestNoReportedCountGoesUnrepaired(t *testing.T) {
	for _, doc := range []string{
		"The Linux syscall takes no flags, and one APE must not answer one call two ways.",
		"It polls every 15 minutes, and the step takes about 90 seconds.",
		"The read has 20 seconds. It carries 500 lines.",
		"It has fewer than 4 rules, and it keeps at least 3 reviewers.",
	} {
		assert.Len(t, Edits(doc, Gate(doc)), len(Gate(doc)), doc)
		assert.Len(t, Edits(doc, Check(doc)), len(Check(doc)), doc)
	}
}

func TestStripLeavesACleanDocumentUntouched(t *testing.T) {
	doc := "Every plugin this repo installs rides in the payload.\n"
	out, cut := Strip(doc)
	assert.Equal(t, doc, out)
	assert.Empty(t, cut)
}

func TestAHitNamesItsLine(t *testing.T) {
	hits := Check("Intro line.\n\nThere are three sections.\n")
	require.Len(t, hits, 1)
	assert.Equal(t, "There are three sections.", hits[0].Line)
	assert.Equal(t, 3, hits[0].LineNo)
}
