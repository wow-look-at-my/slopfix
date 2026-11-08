package slopfix

import _ "embed"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/markdown"
)

//go:embed testdata/readme-tables-and-fences.md
var readmeTablesAndFences string

//go:embed testdata/readme-prose-bullets.md
var readmeProseBullets string

//go:embed testdata/notes-doc-bullets.md
var notesDocBullets string

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

// A table row that describes its member and a fenced line are content. No
// rule reads them as a list, and a repair keeps each byte for byte.
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

// realDocs are grok-build documents. Each holds reference tables, fences and
// prose bullets that must survive, beside enumerations that must go.
var realDocs = []string{
	"assets-README.md",
	"sandbox-jail-notes.md",
	"output-rate-floor-notes.md",
	"local-runtime-notes-ollama-and-lm-studio.md",
}

// The repair of a real document deletes only its enumerations. The golden
// beside each document is what it keeps. SLOPFIX_UPDATE_GOLDEN=1 writes the
// golden again.
func TestARepairOfARealDocumentDeletesOnlyEnumerations(t *testing.T) {
	for _, name := range realDocs {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", "grok-build", name))
			require.NoError(t, err)
			doc := string(src)
			got := counts.ListGate(doc, counts.ListEdits(doc), edit.Scope{})
			require.Empty(t, got.Refused, "the gate refused an edit")

			golden := filepath.Join("testdata", "grok-build", strings.TrimSuffix(name, ".md")+".fixed")
			if os.Getenv("SLOPFIX_UPDATE_GOLDEN") == "1" {
				require.NoError(t, os.WriteFile(golden, []byte(got.Text), 0o644))
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err)
			assert.Equal(t, string(want), got.Text)

			assert.Equal(t, fenceLines(doc), fenceLines(got.Text), "a fenced line went")
			assert.Equal(t, strings.Count(doc, "\n#"), strings.Count(got.Text, "\n#"), "a heading went")
			assert.Empty(t, counts.FindLists(got.Text), "the repair left an enumeration")
		})
	}
}

// One repair pass leaves no enumeration in a markdown case of
// counts/inventory-list: each is cut, or its container goes.
func TestOneRepairPassLeavesNoEnumerationInACase(t *testing.T) {
	for _, rule := range AllRuleSpecs() {
		if rule.ID != counts.IDList {
			continue
		}
		for _, c := range rule.Cases {
			if !strings.HasSuffix(c.Path, ".md") {
				continue
			}
			got := counts.ListGate(c.Text, counts.ListEdits(c.Text), edit.Scope{})
			assert.Empty(t, got.Refused, "case %q: the gate refused an edit", c.Name)
			assert.Empty(t, counts.FindLists(got.Text), "case %q: an enumeration is left", c.Name)
		}
	}
}

// fenceLines answers every line inside a fence, the fence lines included.
func fenceLines(doc string) []string {
	var out []string
	in := ""
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case in == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			in = trimmed[:3]
			out = append(out, line)
		case in != "":
			out = append(out, line)
			if strings.HasPrefix(trimmed, in) && strings.Trim(trimmed, in[:1]) == "" {
				in = ""
			}
		}
	}
	return out
}

// A notes file that is one heading and one list of prose items is the whole
// document. A repair that reads the list as an inventory writes an empty file.
func TestARepairKeepsANotesFileThatIsOneList(t *testing.T) {
	items := strings.Count(notesDocBullets, "\n- ")
	require.Greater(t, items, 1)

	got := Fix(Request{Content: notesDocBullets, Path: "docs/sandbox-jail-notes.md", MaxCommentLines: 14})
	assert.True(t, strings.HasPrefix(got.Text, "# `--sandbox` jail notes\n"), "the heading went: %q", got.Text)
	assert.Equal(t, items, strings.Count(got.Text, "\n- "), "removed %q", got.Removed)
}
