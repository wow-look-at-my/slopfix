package slopfix_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/markdown"
)

// longText repeats sentences past the cap. A parenthesis and a code span with sentences inside them sit in the middle.
func longText() string {
	var b strings.Builder
	for i := range 40 {
		if i == 20 {
			b.WriteString("The feed (it drops a kind. Then it caps the page. The cap bounds the result) serves the page. ")
			b.WriteString("The span `a. B. c` stays whole. ")
		}
		b.WriteString("The runner reads each event from the stream and writes the result to the store. ")
	}
	return strings.TrimSpace(b.String())
}

func docIDs(content string) []string {
	var out []string
	for _, f := range slopfix.Check(content) {
		out = append(out, f.ID)
	}
	return out
}

func TestLongBlockDividesAListItem(t *testing.T) {
	doc := "# Title\n\n- **Admin port**: " + longText() + "\n- The next item.\n\nAfter the list.\n"
	require.Contains(t, docIDs(doc), slopfix.IDLongBlock, "the control: the item is over the cap")

	got := slopfix.Fix(slopfix.Request{Content: doc, Path: "docs/x.md", Rules: []slopfix.Rule{slopfix.RuleWrap}})
	require.True(t, got.Changed)
	assert.Empty(t, got.Refused)
	assert.NotContains(t, docIDs(got.Text), slopfix.IDLongBlock, got.Text)
	assert.True(t, markdown.WordsOnly(doc, got.Text), "a division moves only newlines and blanks")
	assert.Contains(t, got.Text, "\n\n  The ", "a later part is indented to the item's content")
	assert.Contains(t, got.Text, "`a. B. c`", "a code span stays whole")
	assert.Contains(t, got.Text, "- The next item.\n", "the next item stays an item")
	for _, block := range markdown.Split(got.Text) {
		if block.Kind == markdown.Prose {
			assert.LessOrEqual(t, len([]rune(block.Text())), slopfix.LongBlockCap)
		}
	}
}

func TestLongBlockDividesAParagraph(t *testing.T) {
	doc := longText() + "\n"
	require.Contains(t, docIDs(doc), slopfix.IDLongBlock)
	got := slopfix.Fix(slopfix.Request{Content: doc, Path: "x.md", Rules: []slopfix.Rule{slopfix.RuleWrap}})
	assert.NotContains(t, docIDs(got.Text), slopfix.IDLongBlock)
	assert.True(t, markdown.WordsOnly(doc, got.Text))
	assert.Contains(t, got.Text, "store.\n\nThe ")
}

// The division must not leave a part that a paragraph word cap refuses. Under the character target alone, this item left a 126-word paragraph.
func TestLongBlockPartsStayUnderTheWordCap(t *testing.T) {
	raw, err := os.ReadFile("testdata/wasm-dispatch.md.in")
	require.NoError(t, err)
	doc := string(raw)
	require.Contains(t, docIDs(doc), slopfix.IDLongBlock)
	got := slopfix.Fix(slopfix.Request{Content: doc, Path: "x.md", Rules: []slopfix.Rule{slopfix.RuleWrap}})
	assert.Empty(t, got.Refused)
	assert.True(t, markdown.WordsOnly(doc, got.Text))
	parts := 0
	for _, block := range markdown.Split(got.Text) {
		if block.Kind == markdown.Prose {
			parts++
			assert.LessOrEqual(t, len(strings.Fields(block.Text())), slopfix.LongBlockWordCap, block.Text())
		}
	}
	assert.Greater(t, parts, 1)
}

// A short block is never a finding, however many sentences it holds.
func TestLongBlockLeavesAShortBlock(t *testing.T) {
	assert.NotContains(t, docIDs("- A short item. It reads fine.\n"), slopfix.IDLongBlock)
}

// A part never opens with text that a line start reads as a new block.
func TestLongBlockNeverOpensABlock(t *testing.T) {
	text := strings.Repeat("The step runs. 2. Then it stops. # A mark here. ", 60)
	got := slopfix.Fix(slopfix.Request{Content: "- " + text + "\n", Path: "x.md", Rules: []slopfix.Rule{slopfix.RuleWrap}})
	assert.Empty(t, got.Refused)
	assert.NotContains(t, docIDs(got.Text), slopfix.IDLongBlock)
	for _, line := range strings.Split(got.Text, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(trimmed, "2.") || strings.HasPrefix(trimmed, "#"), line)
	}
}
