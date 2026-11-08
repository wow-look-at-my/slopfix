// inventory_list_plan.go holds the inventory list plan code.

package counts

import (
	"sort"

	"github.com/wow-look-at-my/slopfix/edit"
)

// ListHit is an enumeration in a document.
type ListHit struct {
	// LineNo is the line the enumeration opens on, counting from one.
	LineNo int
	Detail string
}

// plan is what the repair takes out of a document: whole lines, and sentences inside a line.
type plan struct {
	hits    []ListHit
	covered []bool
	cuts    []Span
}

// FindLists answers every enumeration in a document.
func FindLists(content string) []ListHit { return planLists(content).hits }

// ListEdits answers the edits that delete every enumeration in a document.
func ListEdits(content string) []edit.Edit {
	p := planLists(content)
	if len(p.hits) == 0 {
		return nil
	}
	var out []edit.Edit
	for i := 0; i < len(p.covered); {
		if !p.covered[i] {
			i++
			continue
		}
		j := i
		for j+1 < len(p.covered) && p.covered[j+1] {
			j++
		}
		out = append(out, edit.Lines(content, i, j, 0, nil))
		i = j + 1
	}
	out = append(out, cutEdits(content, p)...)
	return out
}

// cutEdits answers an edit per sentence cut that no deleted line holds, with
// the blank that divides it from its neighbour.
func cutEdits(content string, p plan) []edit.Edit {
	starts := lineOffsets(content)
	var out []edit.Edit
	for _, c := range p.cuts {
		if p.covered[lineAt(starts, c.Start)] {
			continue
		}
		start, end := c.Start, c.End
		after := end
		for after < len(content) && (content[after] == ' ' || content[after] == '\t') {
			after++
		}
		if after < len(content) && content[after] != '\n' {
			end = after
		} else {
			for start > 0 && (content[start-1] == ' ' || content[start-1] == '\t') {
				start--
			}
		}
		out = append(out, edit.Edit{Start: start, End: end, Cut: []string{content[c.Start:c.End]}})
	}
	return out
}

func lineOffsets(content string) []int {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineAt answers the line, counting from zero, that holds byte at.
func lineAt(starts []int, at int) int {
	return sort.Search(len(starts), func(i int) bool { return starts[i] > at }) - 1
}
