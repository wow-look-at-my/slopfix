// inventory_list_blocks.go holds the inventory list blocks code.

package counts

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/markdown"
)

// proseBlock is a paragraph or a list item, with its lines from zero and the
// bytes of its prose, past its indentation and its marker.
type proseBlock struct {
	first, last int
	start, end  int
	indent      int
	item        bool
	text        string
}

func proseBlocks(content string) []proseBlock {
	starts := lineOffsets(content)
	var out []proseBlock
	for _, b := range markdown.Split(content) {
		if b.Kind != markdown.Prose {
			continue
		}
		first, last := b.Start-1, b.Start-2+len(b.Lines)
		lead := len(b.Lines[0]) - len(strings.TrimLeft(b.Lines[0], " \t"))
		if b.Marker != "" {
			lead += len(b.Marker)
			for lead < len(b.Lines[0]) && b.Lines[0][lead] == ' ' {
				lead++
			}
		}
		end := starts[last] + len(b.Lines[len(b.Lines)-1])
		out = append(out, proseBlock{
			first: first, last: last,
			start: starts[first] + lead, end: end,
			indent: len(b.Indent), item: b.Marker != "",
			text: b.Text(),
		})
	}
	return out
}

// listGroups answers the items of each markdown list, at one indentation.
func listGroups(lines []string, blocks []proseBlock) [][]proseBlock {
	var out [][]proseBlock
	for i := 0; i < len(blocks); i++ {
		if !blocks[i].item {
			continue
		}
		group := []proseBlock{blocks[i]}
		for j := i + 1; j < len(blocks); j++ {
			b := blocks[j]
			if !b.item || b.indent < group[0].indent {
				break
			}
			if b.indent > group[0].indent {
				continue
			}
			if !between(lines, group[len(group)-1].last+1, b.first, group[0].indent) {
				break
			}
			group = append(group, b)
			i = j
		}
		out = append(out, group)
	}
	return out
}

// between reports whether every line from from up to to is blank or indented past indent.
func between(lines []string, from, to, indent int) bool {
	for _, line := range lines[from:to] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " \t")) <= indent {
			return false
		}
	}
	return true
}

// groupEnd answers the last line of a list, nested lines included.
func groupEnd(lines []string, group []proseBlock) int {
	last := group[len(group)-1].last
	indent := group[0].indent
	for i := last + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " \t")) <= indent {
			break
		}
		last = i
	}
	return last
}

// tidyBlanks covers the blank line after each deleted run when a blank line
// already sits before it, so no run of blank lines is left.
func tidyBlanks(lines []string, covered []bool) {
	blank := func(i int) bool { return strings.TrimSpace(lines[i]) == "" }
	for i := 0; i < len(lines); i++ {
		if !covered[i] {
			continue
		}
		j := i
		for j+1 < len(lines) && covered[j+1] {
			j++
		}
		before := i == 0 || blank(i-1)
		switch {
		case j+1 < len(lines)-1 && before && blank(j+1):
			covered[j+1] = true
		case j+1 == len(lines)-1 && i > 0 && blank(i-1) && lines[j+1] == "":
			covered[i-1] = true
		}
		i = j + 1
	}
}

// ListGate writes the deletions the repair answers. Each edit only deletes.
// Whole lines that are not a fence line or a heading, or bytes inside one
// prose block. One table cell or the words of one heading. The result must
// keep every other verbatim line of the source, in the same order, and every
// heading. Each table it keeps must keep its header, its rule and a body row.
func ListGate(content string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(content, scope)
	}
	lines := strings.Split(content, "\n")
	starts := lineOffsets(content)
	blocks := proseBlocks(content)
	st := scan(lines)
	want := shapeOf(content)
	return edit.Gate(content, edits, scope,
		func(e edit.Edit) string {
			if e.Text != "" {
				return "it writes text, and the repair only deletes"
			}
			if from, to, ok := wholeLines(content, starts, e); ok {
				for i := from; i <= to; i++ {
					if st.fence[i] || st.heading[i] {
						return "it deletes a fence line or a heading"
					}
				}
				return ""
			}
			for _, b := range blocks {
				if e.Start >= starts[b.first] && e.End <= b.end {
					return ""
				}
			}
			i := lineAt(starts, e.Start)
			if lineAt(starts, e.End) != i && e.End != starts[i]+len(lines[i]) {
				return "it reaches past its line"
			}
			s, t := e.Start-starts[i], e.End-starts[i]
			if st.table[i] {
				for _, c := range cells(lines[i]) {
					if s >= c[0] && t <= c[1] {
						return ""
					}
				}
			}
			if st.heading[i] && s >= headingText(lines[i]) {
				return ""
			}
			return "it reaches past its prose block, its cell or its heading"
		},
		func(text string) bool { return shapeOf(text) == want })
}

// wholeLines answers the lines, from zero, that e deletes whole.
func wholeLines(content string, starts []int, e edit.Edit) (int, int, bool) {
	if e.End != len(content) && !slices.Contains(starts, e.End) {
		return 0, 0, false
	}
	start := e.Start
	// The last line has no line end of its own, so the one before it goes with it.
	if e.End == len(content) && start < len(content) && content[start] == '\n' {
		start++
	}
	if !slices.Contains(starts, start) || start >= e.End {
		return 0, 0, false
	}
	return lineAt(starts, start), lineAt(starts, e.End-1), true
}

// shapeOf answers what the repair must keep of a document: each verbatim line
// that no table holds. This also covers the count of its headings, and each
// table that is short of a body row. A row that holds a pipe outside a table
// counts too.
func shapeOf(content string) string {
	lines := strings.Split(content, "\n")
	st := scan(lines)
	var b strings.Builder
	for _, blk := range markdown.Split(content) {
		if blk.Kind != markdown.Verbatim {
			continue
		}
		for k, line := range blk.Lines {
			i := blk.Start - 1 + k
			switch {
			case strings.TrimSpace(line) == "" || st.table[i]:
			case st.heading[i]:
				b.WriteString("#\n")
			default:
				b.WriteString(line + "\n")
			}
		}
	}
	for _, t := range st.tables {
		if t[1] < t[0]+2 {
			b.WriteString("table without a body row\n")
		}
	}
	for i, line := range lines {
		if piped(line) && !st.table[i] && !st.fence[i] {
			b.WriteString("row outside a table\n")
		}
	}
	return b.String()
}
