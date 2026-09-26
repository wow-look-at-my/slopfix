package markdown

import (
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

// FormatEdits answers an edit per prose block that joining it, or repair,
// changes.
func FormatEdits(content string, repair func(string) string) []edit.Edit {
	var out []edit.Edit
	for _, block := range Split(content) {
		if block.Kind == Verbatim {
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

// WordsOnly reports whether both documents carry the same words in the same
// order, which proves a rewrite moved newlines and nothing else.
func WordsOnly(before, after string) bool {
	return strings.Join(strings.Fields(before), " ") == strings.Join(strings.Fields(after), " ")
}
