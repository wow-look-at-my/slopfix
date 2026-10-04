// scalars.go answers which rows of a workflow hold a block scalar's own text,
// and which of those rows are a step's script.
//
// A row inside a block scalar is not YAML. A # there opens a shell comment, a
// deeper-indented row there ends nothing, and an editor that reads either as
// syntax rewrites a script. So the rows come from the parser's positions.
package workflow

import (
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// scriptRows is a step's script: the rows it occupies, and the text of each.
type scriptRows struct {
	// start is the 1-based row the script's earliest line sits on.
	start int
	// lines are the rows as the file holds them, indentation included.
	lines []string
}

// blockScalarRows reports, per 0-based row, whether the row holds a block
// scalar's text. The header row itself is YAML and reports false.
func blockScalarRows(content string) []bool {
	inside := make([]bool, len(lines(content)))
	for _, at := range blockScalars(content) {
		for row := at.from; row < at.to && row < len(inside); row++ {
			inside[row] = true
		}
	}
	return inside
}

// rowSpan is a half-open range of 0-based rows.
type rowSpan struct {
	from, to int
}

// blockScalars answers the text rows of every block scalar in the document.
//
// A literal scalar keeps its line breaks, so the parser's own value says how
// many rows it took. A folded scalar does not, and the row after its last is
// the row the next node starts on: YAML reads the rest of the document from
// the earliest row that is not the scalar's.
func blockScalars(content string) map[int]rowSpan {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	end := len(lines(content))
	var starts []int
	var folded []*yaml.Node
	out := make(map[int]rowSpan)
	walk(&doc, func(node *yaml.Node) {
		starts = append(starts, node.Line)
		switch node.Style {
		case yaml.LiteralStyle:
			out[node.Line] = rowSpan{from: node.Line, to: node.Line + countRows(node.Value)}
		case yaml.FoldedStyle:
			folded = append(folded, node)
		}
	})
	for _, node := range folded {
		out[node.Line] = rowSpan{from: node.Line, to: nextStart(starts, node.Line, end)}
	}
	return out
}

// countRows answers how many rows a literal scalar's value took. Its last line
// break ends the last row rather than starting another.
func countRows(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(value, "\n"), "\n") + 1
}

// nextStart answers the row the earliest node after line starts on, or the end
// of the document when the scalar is the last thing in it.
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

// scripts answers every step's run script, read off the parser rather than off
// a search for a run: key, which a script quoting that text would answer too.
func scripts(content string) []scriptRows {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	rows := lines(content)
	spans := blockScalars(content)
	jobs := mappingValue(rootOf(&doc), "jobs")
	if jobs == nil {
		return nil
	}
	var out []scriptRows
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		steps := mappingValue(jobs.Content[i+1], "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			run := mappingValue(step, "run")
			if run == nil || run.Kind != yaml.ScalarNode {
				continue
			}
			out = append(out, scriptOf(run, rows, spans))
		}
	}
	return out
}

// scriptOf answers the rows a run scalar occupies. A script written on the
// run: line itself occupies the row the parser puts it on.
func scriptOf(run *yaml.Node, rows []string, spans map[int]rowSpan) scriptRows {
	at, held := spans[run.Line]
	if !held {
		return scriptRows{start: run.Line, lines: []string{strings.TrimSpace(run.Value)}}
	}
	var body []string
	for row := at.from; row < at.to && row < len(rows); row++ {
		body = append(body, rows[row])
	}
	// A trailing run of blank rows belongs to the document, not the script.
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	return scriptRows{start: at.from + 1, lines: body}
}
