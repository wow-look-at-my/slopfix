package slopfmt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatJoinsAParagraph(t *testing.T) {
	got, safe := Format("A sentence that the author\nwrapped across two lines.\n")
	require.True(t, safe)
	assert.Equal(t, "A sentence that the author wrapped across two lines.\n", got)
}

func TestFormatLeavesAFenceAlone(t *testing.T) {
	// A fence is data. Joining its lines would change what the code does.
	src := "```go\nif err != nil {\n\treturn err\n}\n```\n"
	got, safe := Format(src)
	require.True(t, safe)
	assert.Equal(t, src, got)
}

func TestFormatLeavesTablesAndHeadingsAlone(t *testing.T) {
	src := "# Heading\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	got, safe := Format(src)
	require.True(t, safe)
	assert.Equal(t, src, got)
}

func TestFormatJoinsAListItemAndKeepsItsMarker(t *testing.T) {
	src := "- an item whose text\n  wrapped onto a second line\n- a second item\n"
	got, safe := Format(src)
	require.True(t, safe)
	assert.Equal(t, "- an item whose text wrapped onto a second line\n- a second item\n", got)
}

func TestFormatKeepsNestedIndentation(t *testing.T) {
	src := "- outer\n  - inner text\n    wrapped here\n"
	got, safe := Format(src)
	require.True(t, safe)
	assert.Equal(t, "- outer\n  - inner text wrapped here\n", got)
}

func TestFormatIsIdempotent(t *testing.T) {
	src := "One paragraph, wrapped\nover lines.\n\n```\ncode\n```\n\n- item one\n  continued\n"
	once, safe := Format(src)
	require.True(t, safe)
	twice, safe := Format(once)
	require.True(t, safe)
	assert.Equal(t, once, twice)
}

func TestFormatLosesNoWords(t *testing.T) {
	src := "# Title\n\nSome prose\nwrapped here.\n\n- a list item\n  continued on\n  three lines\n\n```\nverbatim\n```\n"
	got, safe := Format(src)
	require.True(t, safe, "a rewrite that changes words must not be written back")
	assert.Equal(t, strings.Fields(src), strings.Fields(got))
}

func TestCheckReportsAWrappedParagraph(t *testing.T) {
	findings := Check("A sentence that the author\nwrapped across two lines.\n")
	require.Len(t, findings, 1)
	assert.Equal(t, "a paragraph is one line", findings[0].Rule)
	assert.Equal(t, 1, findings[0].Line)
}

func TestCheckIgnoresAFencesContents(t *testing.T) {
	// The shell inside a fence carries semicolons and contractions. It is code.
	src := "```sh\necho one; echo two   # don't touch this\n```\n"
	assert.Empty(t, Check(src))
}

func TestCheckIgnoresInlineCode(t *testing.T) {
	// A semicolon inside a code span is the thing being documented, not prose.
	assert.Empty(t, Check("Write `a; b` on one line.\n"))
}

func TestCheckReportsTheProseRules(t *testing.T) {
	cases := map[string]struct {
		text string
		rule string
	}{
		"contraction":  {"The runner doesn't care.\n", "STE bans contractions"},
		"banned modal": {"The caller should write a period.\n", "STE bans this modal"},
		"semicolon":    {"The runner cares; the caller does not.\n", "STE bans the semicolon"},
		"comma splice": {"The command failed, so the run stops.\n", "a comma joining two clauses is the semicolon STE bans, spelled differently"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			findings := Check(tc.text)
			require.NotEmpty(t, findings)
			assert.Equal(t, tc.rule, findings[0].Rule)
		})
	}
}

func TestCheckReportsALongSentence(t *testing.T) {
	long := "This sentence runs well past the cap because it keeps adding clause after clause " +
		"and never stops to let the reader take a breath before the next idea arrives.\n"
	findings := Check(long)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0].Rule, "sentence cap")
}

func TestCheckAcceptsCompliantProse(t *testing.T) {
	src := "# Title\n\nThe runner reads the file. It reports one finding per rule.\n\n" +
		"- Each item is one line.\n- The reader's window wraps it.\n\n```\ncode; here\n```\n"
	assert.Empty(t, Check(src))
}
