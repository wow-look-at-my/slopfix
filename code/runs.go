// runs.go exposes the comment runs a grammar finds, for a caller that judges a
// comment's words rather than its size.
//
// The parse is the only reader of syntax here. A scanner that looks for a
// marker byte needs a table for each language. The table names the marker
// spelling, the quotes that start a literal, and the quotes that honour a
// backslash. A grammar already
// knows, and a marker inside a string is not a comment node.
package code

import (
	"strings"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// Run is a run of comment lines and where it sits.
type Run struct {
	Start int
	End   int
	Pure  []bool
	// Prose holds each line of the run with every byte outside a comment node written as a blank.
	Prose []string
}

// Runs returns every comment run in src, in file order.
//
// ok is false when no grammar parses the file, or when the parse carries an
// error. A file mid-edit is the common case for a hook, and half a tree reads
// code as prose.
func Runs(filename, src string) (runs []Run, ok bool) {
	root, ok := Parse(filename, src)
	if !ok {
		return nil, false
	}
	var nodes []ts.Node
	gatherComments(root, &nodes)
	if len(nodes) > 0 && IsInterpreter(nodes[0], src) {
		nodes = nodes[1:]
	}
	byStartLine(nodes)
	return merge(nodes, Lines(src)), true
}

func gatherComments(node ts.Node, out *[]ts.Node) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if IsComment(child) {
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
		end := lastLine(nodes[i])
		j := i
		for j+1 < len(nodes) && int(nodes[j+1].StartPoint().Row) <= end+1 && followsCode(nodes[j+1], lines) == followsCode(nodes[j], lines) {
			j++
			if lineNo := lastLine(nodes[j]); lineNo > end {
				end = lineNo
			}
		}
		if start >= 0 && end < len(lines) && start <= end {
			runs = append(runs, Run{
				Start: start,
				End:   end + 1,
				Pure:  purity(nodes[i:j+1], start, end, lines),
				Prose: prose(nodes[i:j+1], start, end, lines),
			})
		}
		i = j + 1
	}
	return runs
}

// prose answers each line of the run with the bytes outside every comment
// node blanked. A reader of the words then sees the comments alone.
func prose(nodes []ts.Node, start, end int, lines []string) []string {
	out := make([]string, end-start+1)
	for i := range out {
		out[i] = strings.Repeat(" ", len(lines[start+i]))
	}
	for _, n := range nodes {
		from, to := int(n.StartPoint().Row), lastLine(n)
		for lineNo := from; lineNo <= to && lineNo <= end; lineNo++ {
			line := lines[lineNo]
			left, right := 0, len(line)
			if lineNo == from {
				left = min(int(n.StartPoint().Column), right)
			}
			if lineNo == int(n.EndPoint().Row) {
				right = min(int(n.EndPoint().Column), right)
			}
			if left >= right {
				continue
			}
			kept := out[lineNo-start]
			out[lineNo-start] = kept[:left] + line[left:right] + kept[right:]
		}
	}
	return out
}

// purity marks the lines a caller may cut. A line shared with code is never
// pure, because deleting it deletes the code.
func purity(nodes []ts.Node, start, end int, lines []string) []bool {
	pure := make([]bool, end-start+1)
	for i := range pure {
		pure[i] = true
	}
	for _, n := range nodes {
		from, to := int(n.StartPoint().Row), lastLine(n)
		if before := int(n.StartPoint().Column); before > 0 && !blankTo(lines, from, before) {
			pure[from-start] = false
		}
		if to == int(n.EndPoint().Row) && !blankFrom(lines, to, int(n.EndPoint().Column)) {
			pure[to-start] = false
		}
	}
	// A line inside the run that no comment covers holds something else.
	covered := make([]bool, len(pure))
	for _, n := range nodes {
		for lineNo := int(n.StartPoint().Row); lineNo <= lastLine(n); lineNo++ {
			covered[lineNo-start] = true
		}
	}
	for i, seen := range covered {
		if !seen {
			pure[i] = false
		}
	}
	return pure
}

// lastLine is the last line that holds part of a comment.
func lastLine(n ts.Node) int {
	end := n.EndPoint()
	if end.Column == 0 && end.Row > n.StartPoint().Row {
		return int(end.Row) - 1
	}
	return int(end.Row)
}

// followsCode reports a comment that shares its first line with code before it. A note on a line of code is not part of the block above or below it.
func followsCode(n ts.Node, lines []string) bool {
	col := int(n.StartPoint().Column)
	return col > 0 && !blankTo(lines, int(n.StartPoint().Row), col)
}

// blankTo reports whether the line holds only whitespace before a column.
func blankTo(lines []string, lineNo, col int) bool {
	if lineNo < 0 || lineNo >= len(lines) || col > len(lines[lineNo]) {
		return false
	}
	return strings.TrimSpace(lines[lineNo][:col]) == ""
}

func blankFrom(lines []string, lineNo, col int) bool {
	if lineNo < 0 || lineNo >= len(lines) {
		return false
	}
	if col > len(lines[lineNo]) {
		return true
	}
	return strings.TrimSpace(lines[lineNo][col:]) == ""
}

// byStartLine puts the comments in file order.
func byStartLine(nodes []ts.Node) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j].StartPoint().Row < nodes[j-1].StartPoint().Row; j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}
