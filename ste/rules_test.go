package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An HTML entity ends in a semicolon. Read as prose, it reports a banned
// semicolon in a sentence that has none.
func TestAnEntityIsNotASemicolon(t *testing.T) {
	assert.Empty(t, Check("The label renders as Analyze&lpar;&rpar; on the node.", 1))
	assert.Empty(t, Check("A non-breaking space is &#160; in the source.", 1))
}

func TestASemicolonInProseIsStillReported(t *testing.T) {
	findings := Check("The runner cares; the caller does not.", 1)
	require.Len(t, findings, 1)
	assert.Equal(t, "STE bans the semicolon", findings[0].Rule)
}

func TestSentencesSplitOnARealBoundary(t *testing.T) {
	got := Sentences("The lexer runs first. The parser runs second.")
	assert.Equal(t, []string{"The lexer runs first.", "The parser runs second."}, got)
}

// A period-and-space split cuts these apart. Each piece then reads as a short
// sentence, and a sentence over the cap goes unreported.
func TestSentencesKeepAnAbbreviationWhole(t *testing.T) {
	for _, text := range []string{
		"The scanner drops a comment, e.g. the shebang line, before it emits a token.",
		"A backtick body nests, vs. a paren body which does not.",
	} {
		assert.Len(t, Sentences(text), 1, text)
	}
}

// The mirror defect: a splitter that demands a capital welds a sentence onto
// its predecessor, because a technical sentence often opens with a file name.
func TestSentencesSplitBeforeALowerCaseFileName(t *testing.T) {
	got := Sentences("Both scanners obey one rule. quoting.md §1.3 binds them to it.")
	assert.Len(t, got, 2)
	got = Sentences("The parser rejects it. §5.2 names the reason.")
	assert.Len(t, got, 2)
}

// A mark after a period closes the sentence only when the sentence opened it.
// The `*` of a glob and a `)` with no open parenthesis are words.
func TestSentencesKeepAGlobAndAStrayCloserWhole(t *testing.T) {
	for _, text := range []string{
		"The package pulls in a couple of esnext.* libs that the step names.",
		"The step reads lib.dom.* when a step passes dom: true.",
	} {
		assert.Len(t, Sentences(text), 1, text)
	}
	got := Sentences("The step reads *every lib.* The next one reads none.")
	assert.Equal(t, []string{"The step reads *every lib.*", "The next one reads none."}, got)
	got = Sentences("The step reads a lib (the dom one.) The next one reads none.")
	assert.Equal(t, []string{"The step reads a lib (the dom one.)", "The next one reads none."}, got)
}

func TestALongSentenceIsReportedOnceTheSplitterIsHonest(t *testing.T) {
	// A single sentence, well over the cap.
	long := "The scanner walks the input once and hands the parser every token it " +
		"needs, e.g. the span and the depth, so that lexer.md and the analyzer " +
		"never disagree about what a word is."
	findings := Check(long, 7)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0].Rule, "sentence cap")
	assert.Equal(t, 7, findings[0].Line)
}

// Text in parentheses counts as a single word, which STE says. A sentence
// full of citations is otherwise reported as too long when it is not.
func TestParenthesesCountAsOneWord(t *testing.T) {
	assert.Equal(t, 5, WordCount("The lexer (see lexer.md §3.3.4) runs first."))
	// A section link holds no word, as the number it replaced held none.
	assert.Equal(t, WordCount(strip("The §9 trigger fires.")), WordCount(strip("The [§owning-renderer](#9-owning-the-renderer) trigger fires.")))
}

// An XML character-class list is a literal, not a sentence. It holds no word the
// cap can weigh, so no division of it is ever a sentence.
func TestACharacterRangeListIsNotProse(t *testing.T) {
	list := "[#xF8-#x2FF] | [#x370-#x37D] | [#x37F-#x1FFF] | [#x200C-#x200D] | " +
		"[#x2070-#x218F] | [#x2C00-#x2FEF] | [#x3001-#xD7FF] | [#xF900-#xFDCF] | " +
		"[#xFDF0-#xFFFD] | [#x10000-#xEFFFF]"
	assert.Equal(t, 0, WordCount(strip(list)))
	assert.Empty(t, Check(list, 37))
	// The control: words beside a range are still weighed.
	assert.Positive(t, WordCount(strip(list+" are the characters a name starts with.")))
}

func TestSplicesTheReferenceRuleReports(t *testing.T) {
	cases := map[string]string{
		"bare":            "The file is short, it fits on one screen.",
		"and":             "The enclosing depths are saved, and they are restored at the boundary.",
		"but":             "The text is not normative, but the content must list every builtin.",
		"so":              "The command failed, so the run stops.",
		"determiner then": "The scan is one pass, so the analyzer keeps every rune.",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			findings := Check(text, 1)
			require.NotEmpty(t, findings)
			assert.Equal(t, "a comma joining two clauses is the semicolon STE bans, spelled differently", findings[0].Rule)
		})
	}
}

// A guard that fires on ordinary English teaches the reader to skim its output.
func TestSplicesLeaveOrdinaryEnglishAlone(t *testing.T) {
	cases := map[string]string{
		"oxford comma":         "The lexer, the parser, and the analyzer read one scan.",
		"list of steps":        "Run the build, then the tests.",
		"introductory phrase":  "When the file is short, it fits on one screen.",
		"imperative after and": "Point it at its predecessor, and repoint the successor at it.",
		"trailing noun phrase": "It reads the tokens, the errors, and the validity flag.",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, Check(text, 1), text)
		})
	}
}

func TestCountsAreReported(t *testing.T) {
	findings := Check("The command carries three redirection fields.", 1)
	require.NotEmpty(t, findings)
	assert.Equal(t, "a stated count goes stale when the set changes", findings[0].Rule)
}

// Arithmetic is not a count of items. A range and an expression name no set,
// so neither goes stale when somebody adds a field.
func TestCountsLeaveArithmeticAlone(t *testing.T) {
	cases := map[string]string{
		"range":      "The exit code range is 0-255 and nothing outside it.",
		"expression": "A chain of N commands carries N-1 operators.",
		"equation":   "It defaults to RGBA8 (32 bits per pixel = 4 bytes), the default target.",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			for _, finding := range Check(text, 1) {
				assert.NotContains(t, finding.Rule, "stated count", text)
			}
		})
	}
}

// A measurement is a stated value the same as a tally is. A size and a duration
// are true today and nothing corrects either when the code around them moves.
func TestCountsReadAMeasurement(t *testing.T) {
	cases := map[string]string{
		"a size":     "The budget is 40000 characters per file.",
		"a duration": "The wait ends after 30 seconds.",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			findings := Check(text, 1)
			require.NotEmpty(t, findings, text)
			assert.Equal(t, "a stated count goes stale when the set changes", findings[0].Rule)
		})
	}
}
