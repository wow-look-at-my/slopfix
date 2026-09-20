// yamlcomments.go finds the comments in a YAML-family file.
//
// A hash opens a comment in both YAML and bash, which is not the same as both
// agreeing on where a single starts. That string is an XML character reference
// in the middle of a scalar, and the file it sat in was a test fixture.
//
// YAML opens a comment on a `#` that starts a line or follows a space, and
// never inside a block scalar, where every line indented past the key is
// content whatever it holds.
package treecomments

import "strings"

// yamlExts are the extensions this scanner reads rather than a grammar.
var yamlExts = map[string]bool{
	".yml":  true,
	".yaml": true,
	".dats": true,
}

// yamlComments returns every comment in a YAML-family source, in source order.
func yamlComments(src string) []Comment {
	var out []Comment
	// blockIndent is the column a block scalar's content must pass to be
	// content. A negative value means no block scalar is open.
	blockIndent := -1
	at := 0
	for number, line := range strings.Split(src, "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		blank := strings.TrimSpace(line) == ""
		switch {
		case blockIndent >= 0 && (blank || indent > blockIndent):
			// Content of an open block scalar, blank lines included.
		case blockIndent >= 0:
			// The indent came back, so the scalar closed on this line.
			blockIndent = -1
			fallthrough
		default:
			if col := yamlCommentAt(line); col >= 0 {
				out = append(out, Comment{
					Text:   line[col:],
					Offset: at + col,
					Line:   number + 1,
					Col:    col,
					Lines:  1,
				})
			} else if opensBlockScalar(line) {
				blockIndent = indent
			}
		}
		at += len(line) + 1
	}
	return out
}

// A `#` inside a quoted scalar is content, and so is a single joined to the
// text before it.
func yamlCommentAt(line string) int {
	quote := byte(0)
	for i := range len(line) {
		c := line[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '#':
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return i
			}
		}
	}
	return -1
}

// opensBlockScalar reports whether the line ends with a literal or folded
// indicator, which makes the deeper-indented lines below it content.
func opensBlockScalar(line string) bool {
	trimmed := strings.TrimRight(line, " \t")
	trimmed = strings.TrimRight(trimmed, "0123456789+-")
	return strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, ">")
}
