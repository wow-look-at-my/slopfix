// Format puts each prose block on a single line, moving newlines only.

package markdown

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"

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
// changes. A run of one-word lines is left to EntryRepair.
func FormatEdits(content string, repair func(string) string) []edit.Edit {
	var out []edit.Edit
	for _, block := range Split(content) {
		if block.Kind == Verbatim || (block.Marker == "" && entryList(block.Lines)) {
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

// EntryRepair answers the edits that give each run of one-word lines a mark
// of its own, in the style named. It answers none unless the result adds
// nothing but that mark to the start of an entry line.
func EntryRepair(content string, style EntryStyle) []edit.Edit {
	var out []edit.Edit
	for _, block := range Split(content) {
		if block.Kind == Prose && block.Marker == "" && entryList(block.Lines) {
			out = append(out, BlockEdit(content, block, entryLines(block, style)))
		}
	}
	if !entryRepairOnly(content, Apply(content, out, edit.Scope{}).Text, style) {
		return nil
	}
	return out
}

// entryRepairOnly reports whether after is before with each entry line marked
// for style. This covers a bullet ahead of its text, or a blank line ahead of
// every entry but the first of a run. Every other line is as written.
func entryRepairOnly(before, after string, style EntryStyle) bool {
	entry := set.New[int]()
	for _, b := range Split(before) {
		if b.Kind == Prose && b.Marker == "" && entryList(b.Lines) {
			for k := range b.Lines {
				entry.Add(b.Start - 1 + k)
			}
		}
	}
	was, now := strings.Split(before, "\n"), strings.Split(after, "\n")
	j := 0
	for i, line := range was {
		if style == EntryBlankLines && entry.Contains(i) && entry.Contains(i-1) {
			if j >= len(now) || now[j] != "" {
				return false
			}
			j++
		}
		if j >= len(now) {
			return false
		}
		got := now[j]
		j++
		switch {
		case !entry.Contains(i):
			if got != line {
				return false
			}
		case style == EntryBullets:
			if strings.TrimSpace(got) != "- "+strings.TrimSpace(line) {
				return false
			}
		default:
			if strings.TrimSpace(got) != strings.TrimSpace(line) {
				return false
			}
		}
	}
	return j == len(now)
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
	return strings.Join(strings.Fields(before), " ") == strings.Join(strings.Fields(after), " ")
}
