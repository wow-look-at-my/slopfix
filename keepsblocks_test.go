package slopfix

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/markdown"
)

//go:embed testdata/readme-tables-and-fences.md
var readmeTablesAndFences string

//go:embed testdata/readme-prose-bullets.md
var readmeProseBullets string

// verbatimRowsOf answers each line, from one, that a fence, a table or a heading holds.
func verbatimRowsOf(doc string) map[int]string {
	out := map[int]string{}
	for _, b := range markdown.Split(doc) {
		if b.Kind != markdown.Verbatim {
			continue
		}
		for k, line := range b.Lines {
			if strings.TrimSpace(line) != "" {
				out[b.Start+k] = line
			}
		}
	}
	return out
}

// keptLines answers the lines of verbatimRowsOf in document order.
func keptLines(doc string) []string {
	var out []string
	for _, b := range markdown.Split(doc) {
		if b.Kind != markdown.Verbatim {
			continue
		}
		for _, line := range b.Lines {
			if strings.TrimSpace(line) != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// A table row and a fenced line are content. No rule reads them as a list,
// and a repair keeps each byte for byte.
func TestARepairKeepsEveryTableRowAndFencedLine(t *testing.T) {
	verbatim := verbatimRowsOf(readmeTablesAndFences)
	require.NotEmpty(t, verbatim)
	for _, f := range CheckContent("README.md", readmeTablesAndFences) {
		_, inBlock := verbatim[f.Line]
		assert.False(t, inBlock, "%s reports line %d, which a table, a fence or a heading holds: %q", f.ID, f.Line, f.Detail)
	}
	assert.Empty(t, counts.FindLists(readmeTablesAndFences), "a table row or a fenced line is not an enumeration")

	got := Fix(Request{Content: readmeTablesAndFences, Path: "README.md", MaxCommentLines: 14})
	assert.Equal(t, keptLines(readmeTablesAndFences), keptLines(got.Text), "removed %q", got.Removed)
}

// A list item of several sentences is prose. A list of them that opens each
// item on a code span keeps every item, and the heading above it.
func TestARepairKeepsAListOfProseItems(t *testing.T) {
	assert.Empty(t, counts.FindLists(readmeProseBullets))
	items := strings.Count(readmeProseBullets, "\n- ")

	got := Fix(Request{Content: readmeProseBullets, Path: "docs/json-config-notes.md", MaxCommentLines: 14})
	assert.True(t, strings.HasPrefix(got.Text, "# JSON config notes\n"), "the heading went: %q", got.Text)
	assert.Equal(t, items, strings.Count(got.Text, "\n- "), "removed %q", got.Removed)
}
