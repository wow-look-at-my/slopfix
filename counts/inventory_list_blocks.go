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

// catalog reports a usage fence. Its code lines open with one word. The word
// after it differs on MinMembers lines or more.
func catalog(body []string) bool {
	seconds := map[string]map[string]bool{}
	for _, line := range body {
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i]
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$ "))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		words := strings.Fields(trimmed)
		if len(words) < 2 {
			continue
		}
		if seconds[words[0]] == nil {
			seconds[words[0]] = map[string]bool{}
		}
		seconds[words[0]][words[1]] = true
	}
	for _, second := range seconds {
		if len(second) >= MinMembers {
			return true
		}
	}
	return false
}

// tableList reports a table that lists members. One column holds a member on
// MinMembers rows or more, or one row holds a member in MinMembers cells or more.
func tableList(rows []string) (string, bool) {
	columns := map[int]int{}
	for n, row := range rows {
		if n == 1 {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
		inRow := 0
		for c, cell := range cells {
			if holdsMember(cell) {
				inRow++
				if n > 1 {
					columns[c]++
				}
			}
		}
		if inRow >= MinMembers {
			return strings.TrimSpace(row), true
		}
	}
	for _, count := range columns {
		if count >= MinMembers {
			return strings.TrimSpace(rows[0]), true
		}
	}
	return "", false
}

// coverEmptySections covers each heading whose section the repair empties.
func coverEmptySections(lines []string, st structure, covered []bool) {
	level := func(i int) int {
		trimmed := strings.TrimSpace(lines[i])
		return len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
	}
	for k := len(st.headings) - 1; k >= 0; k-- {
		h := st.headings[k]
		if covered[h] {
			continue
		}
		end := len(lines)
		for _, next := range st.headings[k+1:] {
			if level(next) <= level(h) {
				end = next
				break
			}
		}
		content, cut := false, true
		for i := h + 1; i < end; i++ {
			if strings.TrimSpace(lines[i]) == "" {
				continue
			}
			content = true
			if !covered[i] {
				cut = false
			}
		}
		if content && cut {
			covered[h] = true
		}
	}
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

// ListGate writes the deletions the repair answers. Each edit only deletes:
// whole lines, or bytes inside one prose block. Every verbatim line of the
// result must be a line of the source, in the same order.
func ListGate(content string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(content, scope)
	}
	starts := lineOffsets(content)
	blocks := proseBlocks(content)
	want := verbatimLines(content)
	return edit.Gate(content, edits, scope,
		func(e edit.Edit) string {
			if e.Text != "" {
				return "it writes text, and the repair only deletes"
			}
			if slices.Contains(starts, e.Start) && (e.End == len(content) || slices.Contains(starts, e.End)) {
				return ""
			}
			for _, b := range blocks {
				if e.Start >= starts[b.first] && e.End <= b.end {
					return ""
				}
			}
			return "it reaches past its prose block"
		},
		func(text string) bool { return subsequence(verbatimLines(text), want) })
}

// verbatimLines answers every line the parser keeps as written, in order.
func verbatimLines(content string) []string {
	var out []string
	for _, b := range markdown.Split(content) {
		if b.Kind != markdown.Verbatim {
			continue
		}
		for _, line := range b.Lines {
			if strings.TrimSpace(line) != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// subsequence reports whether every line of got appears in want, in order.
func subsequence(got, want []string) bool {
	i := 0
	for _, line := range got {
		for i < len(want) && want[i] != line {
			i++
		}
		if i == len(want) {
			return false
		}
		i++
	}
	return true
}
