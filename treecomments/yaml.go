package treecomments

import (
	"github.com/wow-look-at-my/go-containers/set"
	"path/filepath"
	"strings"
)

// isYAML reports a YAML file, which the bash grammar reads for its comments.
func isYAML(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yml" || ext == ".yaml"
}

// yamlLineComments starts each comment on a line that opens on `#` at that
// mark. An apostrophe in a YAML value opens a quote for the bash grammar,
// so a comment can start at a `#` in a code span: "every `#[cfg(test)]`".
func yamlLineComments(src string, comments []Comment) []Comment {
	lines := strings.Split(src, "\n")
	starts := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		starts[i] = offset
		offset += len(line) + 1
	}
	seen := set.New[int]()
	out := comments[:0:0]
	for _, c := range comments {
		if c.Line < 1 || c.Line > len(lines) || c.Lines != 1 {
			out = append(out, c)
			continue
		}
		line := lines[c.Line-1]
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent >= len(line) || line[indent] != '#' || c.Col < indent {
			out = append(out, c)
			continue
		}
		if seen.Contains(c.Line) {
			continue
		}
		seen.Add(c.Line)
		c.Col, c.Offset, c.Text = indent, starts[c.Line-1]+indent, strings.TrimRight(line[indent:], "\r")
		out = append(out, c)
	}
	return out
}
