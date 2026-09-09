// runs.go exposes the comment runs a grammar finds, for a caller that judges a
// comment's words rather than its size.
//
// The parse is the only reader of syntax here. A scanner that looks for a
// marker byte has to be told which spelling each language uses, which quotes
// start a literal, and which of those honour a backslash. A grammar already
// knows, and a marker inside a string is not a comment node.
package commentlength

import (
	"strings"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// Run is a run of comment lines and where it sits. Pure says, for each line,
// whether the line holds the comment and nothing else, so a caller knows which
// lines it can cut without taking code with them.
type Run struct {
	Start int
	End   int
	Pure  []bool
}

// Runs returns every comment run in src, in file order.
//
// ok is false when no grammar parses the file, or when the parse carries an
// error. A file mid-edit is the common case for a hook, and half a tree reads
// code as prose.
func Runs(filename, src string) (runs []Run, ok bool) {
	language := languageFor(filename)
	if language == nil {
		return nil, false
	}
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil, false
	}
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return nil, false
	}
	root := tree.RootNode()
	if root.IsNull() || root.HasError() {
		return nil, false
	}

	var nodes []ts.Node
	gatherComments(root, &nodes)
	byStartRow(nodes)
	return merge(nodes, splitLines(src)), true
}

// gatherComments walks the whole tree, so a comment inside a function body is
// found the same way as one above a declaration.
func gatherComments(node ts.Node, out *[]ts.Node) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if isComment(child) {
			*out = append(*out, child)
			continue
		}
		gatherComments(child, out)
	}
}

// merge joins comments on adjoining lines into a run, so a caller measuring
// volume sees the essay rather than each of its lines.
func merge(nodes []ts.Node, lines []string) []Run {
	var runs []Run
	for i := 0; i < len(nodes); {
		start := int(nodes[i].StartPoint().Row)
		end := int(nodes[i].EndPoint().Row)
		j := i
		for j+1 < len(nodes) && int(nodes[j+1].StartPoint().Row) <= end+1 {
			j++
			if row := int(nodes[j].EndPoint().Row); row > end {
				end = row
			}
		}
		if start >= 0 && end < len(lines) && start <= end {
			runs = append(runs, Run{
				Start: start,
				End:   end + 1,
				Pure:  purity(nodes[i:j+1], start, end, lines),
			})
		}
		i = j + 1
	}
	return runs
}

// purity marks the lines a caller may cut. A line shared with code is never
// pure, because deleting it deletes the code.
func purity(nodes []ts.Node, start, end int, lines []string) []bool {
	pure := make([]bool, end-start+1)
	for i := range pure {
		pure[i] = true
	}
	for _, n := range nodes {
		from, to := int(n.StartPoint().Row), int(n.EndPoint().Row)
		if before := int(n.StartPoint().Column); before > 0 && !blankTo(lines, from, before) {
			pure[from-start] = false
		}
		if after := int(n.EndPoint().Column); !blankFrom(lines, to, after) {
			pure[to-start] = false
		}
	}
	// A line inside the run that no comment covers holds something else.
	covered := make([]bool, len(pure))
	for _, n := range nodes {
		for row := int(n.StartPoint().Row); row <= int(n.EndPoint().Row); row++ {
			covered[row-start] = true
		}
	}
	for i, seen := range covered {
		if !seen {
			pure[i] = false
		}
	}
	return pure
}

// blankTo reports whether the line holds only whitespace before a column.
func blankTo(lines []string, row, col int) bool {
	if row < 0 || row >= len(lines) || col > len(lines[row]) {
		return false
	}
	return strings.TrimSpace(lines[row][:col]) == ""
}

// blankFrom reports whether the line holds only whitespace from a column on.
func blankFrom(lines []string, row, col int) bool {
	if row < 0 || row >= len(lines) {
		return false
	}
	if col > len(lines[row]) {
		return true
	}
	return strings.TrimSpace(lines[row][col:]) == ""
}

// byStartRow puts the comments in file order, which is the order a caller
// splices them back in.
func byStartRow(nodes []ts.Node) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j].StartPoint().Row < nodes[j-1].StartPoint().Row; j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}
