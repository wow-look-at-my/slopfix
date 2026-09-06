package markdown

import "strings"

// Format rewrites a document so every prose block is one line.
//
// A wrap is one author's guess at one reader's window, frozen into the file. It
// turns a two-word change into a diff that looks like a rewrite. Joining is the
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
	// Split on "\n" gives a trailing empty element for a file ending in a
	// newline, which the loop above already wrote. Drop the duplicate.
	return strings.TrimSuffix(out.String(), "\n")
}

// WordsOnly reports whether two documents carry the same words in the same
// order. Formatting must only move newlines, so a caller can prove a rewrite
// lost nothing before it writes the file back.
func WordsOnly(before, after string) bool {
	return strings.Join(strings.Fields(before), " ") == strings.Join(strings.Fields(after), " ")
}
