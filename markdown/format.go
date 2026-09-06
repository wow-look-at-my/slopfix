package markdown

import "strings"

// Format puts each prose block on a single line, moving newlines only.
func Format(content string) string {
	return FormatFunc(content, func(prose string) string { return prose })
}

// FormatFunc joins each prose block and passes its text through repair before
// it is written. A verbatim block never reaches repair, so a fence, a table and
// a heading arrive at the caller as they were.
func FormatFunc(content string, repair func(string) string) string {
	var out strings.Builder
	for _, block := range Split(content) {
		if block.Kind == Verbatim {
			for _, line := range block.Lines {
				out.WriteString(line)
				out.WriteByte('\n')
			}
			continue
		}
		out.WriteString(block.Indent)
		if block.Marker != "" {
			out.WriteString(block.Marker)
			out.WriteByte(' ')
		}
		out.WriteString(repair(block.Text()))
		out.WriteByte('\n')
	}
	// The loop already wrote the final newline. Drop the duplicate.
	return strings.TrimSuffix(out.String(), "\n")
}

// WordsOnly reports whether both documents carry the same words in the same
// order, which proves a rewrite moved newlines and nothing else.
func WordsOnly(before, after string) bool {
	return strings.Join(strings.Fields(before), " ") == strings.Join(strings.Fields(after), " ")
}
