package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/ste"
)

// The sentences the repair rewrites, and the sentences it declines, are worked
// examples in the rules folder. What is left here is what such an example
// cannot say: an invariant the repair holds whatever it writes.

// Leading closes a sentence at a clause boundary the parser finds. The head it
// keeps is a sentence under the cap and never a cut at a word.
func TestLeadingClosesAtAClauseBoundary(t *testing.T) {
	long := "The comment scan reads each file that the branch changed since its merge base, and it rewrites every number it finds in a comment into words that stay true."
	head, ok := ste.Leading(long)
	require.True(t, ok)
	assert.Equal(t, "The comment scan reads each file that the branch changed since its merge base.", head)
	assert.LessOrEqual(t, ste.WordCount(head), ste.SentenceWordCap)
	assert.Empty(t, ste.Check(head, 1))

	_, ok = ste.Leading("a very long run of words with no verb and no boundary of any kind at all anywhere in it whatsoever here")
	assert.False(t, ok, "no clause boundary, so no head")
}

// With no clause boundary, the long subject goes into a sentence of its own,
// and the noun it names carries the predicate.
func TestFixDividesASentenceWithNoClauseBoundary(t *testing.T) {
	long := "A reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a repair from the tool rather than a deletion."
	findings := ste.Check(long, 1)
	require.Len(t, findings, 1, "the control: the sentence is over the cap")
	assert.False(t, ste.ByHand(findings[0].Fix), findings[0].Fix)
	assert.Equal(t, "Consider a reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause. That reader still deserves a repair from the tool rather than a deletion.", ste.Fix(long))
}

// A division never lands inside an inline code span, so the span survives the
// repair exactly as the source wrote it.
func TestFixDividesALongSentenceAroundACodeSpan(t *testing.T) {
	long := "The gate reads `a; b` out of every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "`a; b`")
	assert.Empty(t, ste.Check(fixed, 1))
}

// The parser reads a masked code span, whose filler word ends before the closing
// backtick. A division after the span must still keep the whole span.
func TestFixKeepsTheCodeSpanThatEndsTheLeftSentence(t *testing.T) {
	long := "The tarball is rooted at `./` and unpacks *as* the build directory — extracting it without `-C dir` sprays `src/`, `include/` and a foreign `.gitignore` over the repo root and chowns it."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "rooted at `./`")
	assert.Contains(t, fixed, "*as*")
	assert.NotContains(t, fixed, "Unpacks")
}

// A splice whose subject is a code span gets the same repair as a word subject.
func TestFixRepairsASpliceBeforeACodeSpanSubject(t *testing.T) {
	text := "The path decides the dialect by its tail: `/responses` is `DialectResponses`, `/messages` is `DialectAnthropic`, `/chat/completions` is `DialectOpenAI`. The version segment is matched but not read, so a `v2/responses` still answers."
	require.NotEmpty(t, ste.Check(text, 1), "the control: the splice is reported")
	fixed := ste.FixSelected(text, func(id string) bool { return id == ste.IDCommaSplice })
	assert.Contains(t, fixed, "`v2/responses`")
	for _, f := range ste.Check(fixed, 1) {
		assert.NotEqual(t, ste.IDCommaSplice, f.ID, "%s\n%s", fixed, f.Detail)
	}
}

// A division never lands inside bold text, so each marker keeps its partner.
func TestFixNeverDividesInsideBoldText(t *testing.T) {
	long := "With no trip count given, each loop is modeled as one iteration **and the estimate is flagged** with a section in the report and a note in the output so it is never read as the exact cost."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "one iteration. **The estimate is flagged** with", "the bold run moves past the dropped conjunction, whole")
	assert.Equal(t, 2, strings.Count(fixed, "**"))
}

// ", and" before a subordinate clause and its main clause is a sentence boundary.
func TestFixDividesBeforeASubordinateClauseAfterAnd(t *testing.T) {
	long := "This closed a real hole: `a_test.go` is `//go:build x`, and for as long as the gate ran default tags only, its violations were invisible and its tests compiled nowhere."
	fixed := ste.Fix(long)
	assert.Equal(t, "This closed a real hole: `a_test.go` is `//go:build x`. For as long as the gate ran default tags only, its violations were invisible and its tests compiled nowhere.", fixed)
	assert.Empty(t, ste.Check(fixed, 1))

	long = "The gate reads every file that the session wrote, but if the cache is cold at the start of the run, the build waits for the whole tree."
	assert.Equal(t, "The gate reads every file that the session wrote. However, if the cache is cold at the start of the run, the build waits for the whole tree.", ste.Fix(long))
}

// A quotation is another voice, so a rule never judges the words inside it.
func TestCheckSkipsQuotedText(t *testing.T) {
	assert.Empty(t, ste.Check(`The owner said "it is fine; ship it" and moved on.`, 1))
	assert.Empty(t, ste.Check("The owner said “it is fine; ship it” and moved on.", 1))
	assert.NotEmpty(t, ste.Check("The owner said it is fine; ship it.", 1))
}

// A numeral that a verb follows is the noun of its phrase. A cut left "So the cannot disagree".
func TestFixKeepsANumeralThatIsTheNoun(t *testing.T) {
	for _, in := range []string{
		"So the two cannot disagree about what red means.",
		"Another repo's run must not animate this one.",
	} {
		assert.Equal(t, in, ste.Fix(in))
	}
}

// A division restates only the subject of the clause right before the cut, never "The Send".
func TestADivisionNamesNoWrongSubject(t *testing.T) {
	in := "A Send Now delivers its text to the planner already running (`SubagentEvent::Interject`, routed by the coordinator id the spawn publishes on the goal tracker) instead of cancelling it, so an `Interrupted` reaching the loop is a bare cancel and is terminal — retrying one spawned four dead planners in 2.3 s before the attempt cap paused the goal."
	assert.NotContains(t, ste.Fix(in), "The Send is")
}

// After a colon, a division needs a clause on its left. "every `agent()` call." is a list item.
func TestADivisionAfterAColonKeepsAClauseOnItsLeft(t *testing.T) {
	in := "Workflows use an absolute cumulative `agent_budget` cap on logical child-agent calls: every `agent()` call and every item in a `parallel()` panel spends one slot, while schema-correction retries don't."
	assert.NotContains(t, ste.Fix(in), "`agent()` call.")
}

// A division whose first half opens with the same words as the whole is a
// repair. Check does not ask for a rewrite by hand.
func TestCheckAgreesWithADivisionThatKeepsTheOpening(t *testing.T) {
	in := "The loader reads every cached manifest from the shared store and rebuilds the index of plugin hooks for each workspace the user opens in the editor during startup of the session."
	require.NotEqual(t, in, ste.Fix(in))
	for _, f := range ste.Check(in, 1) {
		assert.False(t, ste.ByHand(f.Fix), f.Fix)
	}
}

// Fix leaves a quotation as Check reads it. A modal inside one stays.
func TestFixLeavesQuotedWordsAlone(t *testing.T) {
	in := `IDLE is an ACTIVE corruption, not a benign "would not attach its cost."`
	assert.Equal(t, in, ste.Fix(in))
	assert.Equal(t, "It will not attach.", ste.Fix("It would not attach."))
}

// A semicolon that ends the prose before a code span keeps its space.
func TestFixKeepsTheSpaceBeforeACodeSpan(t *testing.T) {
	assert.Equal(t, "Docker is unavailable. `MESA_DIR` still overrides it.", ste.Fix("Docker is unavailable; `MESA_DIR` still overrides it."))
}

// A parenthetical counts as a single word, so a division inside it would halve
// something STE says is indivisible.
func TestFixDividesALongSentenceAroundAParenthetical(t *testing.T) {
	long := "The gate reads every file in the session (the header, the body and the trailer alike) and refuses the write when any one of them carries a finding nothing repairs."
	fixed := ste.Fix(long)
	assert.Contains(t, fixed, "(the header, the body and the trailer alike)")
	assert.Empty(t, ste.Check(fixed, 1))
}

// A repair that leaves its own finding standing loops the caller forever.
func TestFixClearsTheMechanicalFindings(t *testing.T) {
	text := "It doesn't matter; a caller should wait, so the write fails."
	assert.NotEmpty(t, ste.Check(text, 1))
	assert.Empty(t, ste.Check(ste.Fix(text), 1))
}

// A negation stands between the auxiliary and its participle.
func TestAPerfectTenseWithANegationIsLeftAsWritten(t *testing.T) {
	for _, text := range []string{
		"It has not repeated.",
		"The walk has never finished.",
		"It has not been read.",
	} {
		assert.Empty(t, ste.Check(text, 1), text)
		assert.Equal(t, text, ste.Fix(text), text)
	}
}

// A perfect after a modal has no simple tense the repair can write, so it is
// neither reported nor rewritten.
func TestAPerfectAfterAModalIsLeftAsWritten(t *testing.T) {
	tense := func(id string) bool { return id == ste.IDTense }
	for _, text := range []string{
		"The number says whether a given run can have resolved it at all.",
		"A probe that reads 2.0 there will have detected the change.",
		"The arm delta would have measured nothing.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, tense), text)
		assert.Empty(t, warned(text, ste.IDTense), text)
	}
}

// A symbol between nouns ends the run, and a run of capitalized words is one
// name. Neither shape is a cluster, so neither is rewritten.
func TestSymbolsAndNamesAreNoNounCluster(t *testing.T) {
	cluster := func(id string) bool { return id == ste.IDNounCluster }
	for _, text := range []string{
		"Several rows in the subgroup / binding / API / video families sit well away from their derivations.",
		"Then complete the GCN -> SPIR-V -> HLSL pipeline.",
		"The ramp runs green → orange → amber → red.",
		"The database came from the AMD Vega Instruction Set Architecture PDF.",
		"Need the gates? DOWNLOAD THE PREBUILT MESA TREE now.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, cluster), text)
		assert.Empty(t, warned(text, ste.IDNounCluster), text)
	}
}

// The cluster repair trusts only plain lowercase runs after a determiner. A
// name, a compound or a run with no determiner can hold a word the tagger
// misread, so the repair leaves it as written.
func TestTheClusterRepairTrustsOnlyPlainRuns(t *testing.T) {
	cluster := func(id string) bool { return id == ste.IDNounCluster }
	for _, text := range []string{
		"The AMD Vega file system cache lookup stopped.",
		"Shuffling pixels through a hardware encode/decode round trip costs more.",
		"It shrinks the dep/waitcnt test blast radius.",
		"Several hours of CI log costs nothing.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, cluster), text)
		assert.Empty(t, warned(text, ste.IDNounCluster), text)
	}
	assert.Equal(t, "The lookup of the cache of the gate file system stopped.",
		ste.FixSelected("The gate file system cache lookup stopped.", cluster))
}

// A verb whose base form the endings cannot settle is not rewritten, so
// "measured" never becomes "measurs".
func TestARepairNeverGuessesABaseForm(t *testing.T) {
	keep := func(id string) bool { return id == ste.IDPassive || id == ste.IDTense }
	for _, text := range []string{
		"The resources are measured by several instruments.",
		"The A/B pair is measuring something other than serialization.",
		"This is load-bearing rather than cosmetic.",
		"The resources are measured by the tool and are not interchangeable.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, keep), text)
	}
	assert.Equal(t, "The gate works the file.", ste.FixSelected("The gate is working the file.", keep))
}

// "more" takes no determiner, so "additional" stays after one.
func TestAdditionalStaysAfterADeterminer(t *testing.T) {
	dict := func(id string) bool { return id == ste.IDDictionary }
	for _, text := range []string{
		"The slope is 1142 ns per additional descriptor.",
		"An additional step runs.",
		"The additional cost is small.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, dict), text)
		assert.Empty(t, warned(text, ste.IDDictionary), text)
	}
	assert.Equal(t, "The tool gives more output today.", ste.FixSelected("The tool gives additional output today.", dict))
}

// A code span moves whole, and a "by" that a conjunction separates from the
// verb is not that verb's actor.
func TestThePassiveRepairKeepsEveryWord(t *testing.T) {
	passive := func(id string) bool { return id == ste.IDPassive }
	for _, text := range []string{
		"The count is derived from a literal EXEC write or supplied by `--exec-lanes`.",
		"Probe shaders are committed as text (`shaders/*.spvasm`), assembled to binaries by `spirv-as` alone.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, passive), text)
	}
	assert.Equal(t, "`spirv-as` reads the file from disk.", ste.FixSelected("The file is read from disk by `spirv-as`.", passive))
}

// A word inside a hyphenated compound is not swapped, and a swap never leaves
// "a" before a vowel.
func TestTheDictionarySwapKeepsCompoundsAndArticles(t *testing.T) {
	dict := func(id string) bool { return id == ste.IDDictionary }
	for _, text := range []string{
		"The output is byte-identical to the input.",
		"It is as much a bug as a wrong instruction is.",
	} {
		assert.Equal(t, text, ste.FixSelected(text, dict), text)
		assert.Empty(t, warned(text, ste.IDDictionary), text)
	}
}

// warned answers the warnings of one rule on text.
func warned(text, id string) []ste.Finding {
	var out []ste.Finding
	for _, f := range ste.Warn(text, 1, false) {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}

// The plain perfect and passive repairs still run when no word stands between
// the auxiliary and its participle. The tense repair is a warning, so a caller
// keeps its ID to reach it.
func TestASimplePerfectTenseIsStillRewritten(t *testing.T) {
	tense := func(id string) bool { return id == ste.IDTense }
	assert.Equal(t, "The gate started the task.", ste.FixSelected("The gate has started the task.", tense))
	assert.Equal(t, "The file was read.", ste.FixSelected("The file has been read.", tense))
}
