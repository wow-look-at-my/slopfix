package slopfix

import _ "embed"

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
	src := testFormatLeavesAFenceAloneMd
	got, safe := Format(src)
	require.True(t, safe)
	assert.Equal(t, src, got)
}

func TestFormatLeavesTablesAndHeadingsAlone(t *testing.T) {
	src := testFormatLeavesTablesAndHeadingsAloneMd
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
	src := testFormatIsIdempotentMd
	once, safe := Format(src)
	require.True(t, safe)
	twice, safe := Format(once)
	require.True(t, safe)
	assert.Equal(t, once, twice)
}

func TestFormatLosesNoWords(t *testing.T) {
	src := testFormatLosesNoWordsMd
	got, safe := Format(src)
	require.True(t, safe, "a rewrite that changes words must not be written back")
	assert.Equal(t, strings.Fields(src), strings.Fields(got))
}

func TestCheckReportsAWrappedParagraph(t *testing.T) {
	findings := Check("A sentence that the author\nwrapped in the middle\nof a line, twice.\n")
	require.Len(t, findings, 2)
	for i, f := range findings {
		assert.Equal(t, "a paragraph is one line", f.Rule)
		assert.Equal(t, i+2, f.Line, "one finding on each continuation line")
	}
}

func TestCheckIgnoresAFencesContents(t *testing.T) {
	// The shell inside a fence carries semicolons and contractions. It is code.
	src := testCheckIgnoresAFencesContentsMd
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
		"semicolon":    {"The runner cares; the caller does not.\n", "a semicolon joins sentences that a period keeps apart"},
		"comma splice": {"The command failed, so the run stops.\n", "a comma joins two clauses that each stand alone"},
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
	src := testCheckAcceptsCompliantProseMd
	assert.Empty(t, Check(src))
}

//go:embed testdata/test-format-leaves-a-fence-alone.md
var testFormatLeavesAFenceAloneMd string

//go:embed testdata/test-format-leaves-tables-and-headings-alone.md
var testFormatLeavesTablesAndHeadingsAloneMd string

//go:embed testdata/test-format-is-idempotent.md
var testFormatIsIdempotentMd string

//go:embed testdata/test-format-loses-no-words.md
var testFormatLosesNoWordsMd string

//go:embed testdata/test-check-ignores-a-fences-contents.md
var testCheckIgnoresAFencesContentsMd string

//go:embed testdata/test-check-accepts-compliant-prose.md
var testCheckAcceptsCompliantProseMd string
