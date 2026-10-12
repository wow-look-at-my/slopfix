// scalars.go answers which lines of a workflow hold a block scalar's own text,
// and which of those lines are a step's script.
//
// A line inside a block scalar is not YAML. A # there opens a shell comment, a
// deeper-indented line there ends nothing, and an editor that reads either as
// syntax rewrites a script. So the lines come from the parser's positions.
package workflow

import (
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// scriptLines is a step's script: the lines it occupies, and the text of each.
type scriptLines struct {
	// start is the 1-based number of the script's earliest line.
	start int
	// lines holds each line as the file holds it, indentation included.
	lines []string
}

// blockScalarLines reports, per 0-based line, whether the line holds a block
// scalar's text. The header line itself is YAML and reports false.
func blockScalarLines(content string) []bool {
	inside := make([]bool, len(lines(content)))
	for _, at := range blockScalars(content) {
		for line := at.from; line < at.to && line < len(inside); line++ {
			inside[line] = true
		}
	}
	return inside
}

// lineSpan is a half-open range of 0-based lines.
type lineSpan struct {
	from, to int
}

// blockScalars answers the text lines of every block scalar in the document.
//
// A literal scalar keeps its line breaks, so the parser's own value says how
// many lines it took. A folded scalar does not. The line after its last is the
// line the next node starts on. YAML reads the rest of the document from the
// earliest line that is not the scalar's.
func blockScalars(content string) map[int]lineSpan {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	end := len(lines(content))
	var starts []int
	var folded []*yaml.Node
	out := make(map[int]lineSpan)
	walk(&doc, func(node *yaml.Node) {
		starts = append(starts, node.Line)
		switch node.Style {
		case yaml.LiteralStyle:
			out[node.Line] = lineSpan{from: node.Line, to: node.Line + countLines(node.Value)}
		case yaml.FoldedStyle:
			folded = append(folded, node)
		}
	})
	for _, node := range folded {
		out[node.Line] = lineSpan{from: node.Line, to: nextStart(starts, node.Line, end)}
	}
	return out
}

// countLines answers how many lines a literal scalar's value took. Its last line
// break ends the last line rather than starting another.
func countLines(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(value, "\n"), "\n") + 1
}

// nextStart answers the line the earliest node after line starts on, or the
// end of the document when the scalar is the last thing.
func nextStart(starts []int, line, end int) int {
	found := end
	for _, at := range starts {
		if at > line && at-1 < found {
			found = at - 1
		}
	}
	return found
}

// walk calls visit for every node in the document, parents before children.
func walk(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	if node.Kind != yaml.DocumentNode {
		visit(node)
	}
	for _, child := range node.Content {
		walk(child, visit)
	}
}

// scriptOf answers the lines a run scalar occupies. A script written on the
// run: line itself occupies the line the parser puts it on.
func scriptOf(run *yaml.Node, fileLines []string, spans map[int]lineSpan) scriptLines {
	at, held := spans[run.Line]
	if !held {
		return scriptLines{start: run.Line, lines: []string{strings.TrimSpace(run.Value)}}
	}
	var body []string
	for line := at.from; line < at.to && line < len(fileLines); line++ {
		body = append(body, fileLines[line])
	}
	// A trailing run of blank lines belongs to the document, not the script.
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	return scriptLines{start: at.from + 1, lines: body}
}
