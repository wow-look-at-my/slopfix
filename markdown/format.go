package markdown

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
)

// Format puts each prose block on a single line, moving newlines only.
func Format(content string) string {
	return FormatFunc(content, func(prose string) string { return prose })
}

// FormatFunc joins each prose block and passes its text through repair before
// it is written. A verbatim block never reaches repair, so a fence.
func FormatFunc(content string, repair func(string) string) string {
	return Apply(content, FormatEdits(content, repair), edit.Scope{}).Text
}

// EntryStyle is how a run of one-word lines is repaired.
type EntryStyle int

const (
	// EntryBullets writes each entry as a list item.
	EntryBullets EntryStyle = iota
	// EntryBlankLines parts each entry from the next with a blank line.
	EntryBlankLines
)

// FormatEdits answers an edit per prose block that joining it, or repair,
// changes. A run of one-word lines becomes a list.
func FormatEdits(content string, repair func(string) string) []edit.Edit {
	return FormatEditsWith(content, repair, EntryBullets)
}

// FormatEditsWith is FormatEdits with the repair of a run of one-word lines named.
func FormatEditsWith(content string, repair func(string) string, entries EntryStyle) []edit.Edit {
	var out []edit.Edit
	for _, block := range Split(content) {
		if block.Kind == Verbatim {
			continue
		}
		if block.Marker == "" && entryList(block.Lines) {
			out = append(out, BlockEdit(content, block, entryLines(block, entries)))
			continue
		}
		var line strings.Builder
		line.WriteString(block.Indent)
		if block.Marker != "" {
			line.WriteString(block.Marker)
			line.WriteByte(' ')
		}
		line.WriteString(repair(block.Text()))
		if len(block.Lines) == 1 && block.Lines[0] == line.String() {
			continue
		}
		out = append(out, BlockEdit(content, block, []string{line.String()}))
	}
	return out
}

// entryLines writes the entries of a block, one to a line.
func entryLines(b Block, style EntryStyle) []string {
	var out []string
	for i, line := range b.Lines {
		entry := strings.TrimSpace(line)
		if style == EntryBlankLines {
			if i > 0 {
				out = append(out, "")
			}
			out = append(out, b.Indent+entry)
			continue
		}
		out = append(out, b.Indent+"- "+entry)
	}
	return out
}

// WordsOnly reports whether both documents carry the same words in the same
// order, which proves a rewrite moved newlines and nothing else.
func WordsOnly(before, after string) bool {
	return words(before) == words(after)
}

// words joins the words of a document, without its list bullets.
func words(content string) string {
	return strings.Join(slices.DeleteFunc(strings.Fields(content), func(w string) bool { return w == "-" }), " ")
}
