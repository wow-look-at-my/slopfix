package markdown

import "strings"

// Format rewrites a document so every prose block occupies a single line.
//
// A wrap freezes the author's guess at the reader's window into the file. It
// turns a small change into a diff that looks like a rewrite. Joining is the
// whole transformation: no word is added, removed or reordered, so a formatted
// file differs from its source only in where the newlines were.
func Format(content string) string {
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
		out.WriteString(block.Text())
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
