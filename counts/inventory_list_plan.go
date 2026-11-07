// inventory_list_plan.go holds the inventory list plan code.

package counts

import (
	"slices"
	"sort"
	"strings"

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

// planLists reads the prose of a document for an enumeration. A fence, a table
// and a heading are verbatim: it neither reads nor deletes them.
func planLists(content string) plan {
	lines := strings.Split(content, "\n")
	starts := lineOffsets(content)
	p := plan{covered: make([]bool, len(lines))}
	blocks := proseBlocks(content)
	verbatim := verbatimRows(content, len(lines))
	cover := func(from, to int) {
		for i := from; i <= to && i < len(lines); i++ {
			p.covered[i] = true
		}
	}
	hit := func(line int, detail string) {
		p.hits = append(p.hits, ListHit{LineNo: line + 1, Detail: detail})
	}
	// The sentence that ends in a colon and introduces a block goes with it.
	intro := func(line int) {
		i := line - 1
		for i >= 0 && strings.TrimSpace(lines[i]) == "" {
			i--
		}
		if i < 0 || !strings.HasSuffix(strings.TrimSpace(lines[i]), ":") {
			return
		}
		for _, b := range blocks {
			if i < b.first || i > b.last {
				continue
			}
			spans := SentenceSpans(content[b.start:b.end])
			if len(spans) <= 1 {
				cover(b.first, b.last)
				return
			}
			last := spans[len(spans)-1]
			p.cuts = append(p.cuts, Span{b.start + last.Start, b.start + last.End})
			return
		}
	}

	for _, group := range listGroups(lines, blocks) {
		// A list whose every item is one member is data the document holds. A
		// paths file or an allowlist reads this way, and no prose enumerates it.
		if bareMembers(group) {
			continue
		}
		// A fence or a table inside an item is content. The repair deletes whole
		// lines, so it leaves such a list as written.
		if slices.Contains(verbatim[group[0].first:groupEnd(lines, group)+1], true) {
			continue
		}
		// An item of more than one sentence describes its member. The list is
		// prose, and only the sentence pass below may cut from it.
		if slices.ContainsFunc(group, func(b proseBlock) bool { return len(SentenceSpans(b.text)) > 1 }) {
			continue
		}
		opening := 0
		keys := make([]string, len(group))
		for n, b := range group {
			if OpensWithMember(b.text) {
				opening++
			}
			keys[n], _ = FrameKey(b.text)
		}
		if opening >= MinMembers {
			hit(group[0].first, group[0].text)
			cover(group[0].first, groupEnd(lines, group))
			intro(group[0].first)
			continue
		}
		for _, run := range SameKeyRuns(keys) {
			hit(group[run[0]].first, group[run[0]].text)
			for _, b := range group[run[0]:run[1]] {
				cover(b.first, b.last)
			}
		}
	}
	for _, b := range blocks {
		if p.covered[b.first] {
			continue
		}
		prose := content[b.start:b.end]
		cuts := ProseCuts(prose)
		if len(cuts) == 0 {
			continue
		}
		for _, c := range cuts {
			hit(lineAt(starts, b.start+c.Start), c.Detail)
		}
		if len(cuts) == len(SentenceSpans(prose)) {
			cover(b.first, b.last)
			continue
		}
		for _, c := range cuts {
			p.cuts = append(p.cuts, Span{b.start + c.Start, b.start + c.End})
		}
	}
	if len(p.hits) == 0 {
		return p
	}
	tidyBlanks(lines, p.covered)
	sort.SliceStable(p.hits, func(a, b int) bool { return p.hits[a].LineNo < p.hits[b].LineNo })
	return p
}
