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

// memberOnly reports a list item that is a bare member and no prose around it:
// a path or a name the document holds. memberOnly is not a sentence that
// enumerates what the code holds.
func memberOnly(text string) bool {
	text = strings.TrimLeft(strings.TrimSpace(text), "*_")
	loc := tokenPattern.FindStringIndex(text)
	if loc == nil || loc[0] != 0 || !IsMember(text[loc[0]:loc[1]]) {
		return false
	}
	return strings.Trim(text[loc[1]:], " \t.,;:!?") == ""
}

// bareMembers reports a list whose every item is one member.
func bareMembers(group []proseBlock) bool {
	for _, b := range group {
		if !memberOnly(b.text) {
			return false
		}
	}
	return true
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
// whole lines, or bytes inside one prose block. The result must keep every
// verbatim line of the source, in the same order.
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
		func(text string) bool { return slices.Equal(verbatimLines(text), want) })
}

// verbatimRows marks each line, from zero, that is not blank and that a verbatim block holds.
func verbatimRows(content string, n int) []bool {
	out := make([]bool, n)
	for _, b := range markdown.Split(content) {
		if b.Kind != markdown.Verbatim {
			continue
		}
		for k, line := range b.Lines {
			if i := b.Start - 1 + k; i < n && strings.TrimSpace(line) != "" {
				out[i] = true
			}
		}
	}
	return out
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
