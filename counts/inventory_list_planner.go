// inventory_list_planner.go judges each table row, list item, heading and paragraph of a document.

package counts

import (
	"sort"
	"strings"
)

// planner builds the plan of one document.
type planner struct {
	plan
	content string
	lines   []string
	starts  []int
	blocks  []proseBlock
	st      structure
}

// planLists reads a document for enumerations. A table row or a list item
// that only names things goes, and a table or a list of nothing else goes
// whole. A row or an item that states something stays, and loses only the
// enumerations inside its sentences. A fence is never read.
func planLists(content string) plan {
	lines := strings.Split(content, "\n")
	pl := &planner{
		plan:    plan{covered: make([]bool, len(lines))},
		content: content, lines: lines,
		starts: lineOffsets(content), blocks: proseBlocks(content), st: scan(lines),
	}
	for _, t := range pl.st.tables {
		pl.table(t)
	}
	for _, group := range listGroups(lines, pl.blocks) {
		pl.list(group)
	}
	for i, line := range lines {
		if pl.st.heading[i] {
			at := headingText(line)
			pl.inline(inlineCuts(line[at:], pl.starts[i]+at))
		}
	}
	for _, b := range pl.blocks {
		pl.prose(b)
	}
	if len(pl.hits) == 0 {
		return pl.plan
	}
	tidyBlanks(lines, pl.covered)
	sort.SliceStable(pl.hits, func(a, b int) bool { return pl.hits[a].LineNo < pl.hits[b].LineNo })
	return pl.plan
}

func (pl *planner) cover(from, to int) {
	for i := from; i <= to && i < len(pl.lines); i++ {
		pl.covered[i] = true
	}
}

func (pl *planner) hit(line int, detail string) {
	pl.hits = append(pl.hits, ListHit{LineNo: line + 1, Detail: detail})
}

// inline takes out each cut inside a line that stays.
func (pl *planner) inline(cuts []ProseCut) {
	for _, c := range cuts {
		pl.hit(lineAt(pl.starts, c.Start), c.Detail)
		pl.cuts = append(pl.cuts, c.Span)
	}
}

// intro covers the sentence that ends in a colon and introduces the block at line.
func (pl *planner) intro(line int) {
	i := line - 1
	for i >= 0 && strings.TrimSpace(pl.lines[i]) == "" {
		i--
	}
	if i < 0 || !strings.HasSuffix(strings.TrimSpace(pl.lines[i]), ":") {
		return
	}
	b := pl.blockAt(i)
	if b == nil {
		return
	}
	// An item that leads in to a list keeps any other content nested under it.
	if b.item {
		for k := b.last + 1; k <= groupEnd(pl.lines, []proseBlock{*b}); k++ {
			if strings.TrimSpace(pl.lines[k]) != "" && !pl.covered[k] {
				return
			}
		}
	}
	spans := SentenceSpans(pl.content[b.start:b.end])
	if len(spans) <= 1 {
		pl.cover(b.first, b.last)
		return
	}
	last := spans[len(spans)-1]
	pl.cuts = append(pl.cuts, Span{b.start + last.Start, b.start + last.End})
}

// table deletes the rows of t that only name things, when they name
// MinMembers members or more. It deletes t whole when no other row is left.
func (pl *planner) table(t [2]int) {
	rows := tableRows(pl.lines, pl.starts, t)
	names, entries := 0, 0
	for _, r := range rows {
		if r.names > 0 {
			names += r.names
			entries++
		}
	}
	if names >= MinMembers && entries == len(rows) {
		pl.hit(t[0], strings.TrimSpace(pl.lines[rows[0].line]))
		pl.cover(t[0], t[1])
		pl.intro(t[0])
		return
	}
	for _, r := range rows {
		if r.names > 0 && names >= MinMembers {
			pl.hit(r.line, strings.TrimSpace(pl.lines[r.line]))
			pl.cover(r.line, r.line)
			continue
		}
		pl.inline(r.cuts)
	}
}

// list deletes the items of group that only name things, when they name
// MinMembers members or more, and each run of items that differ only in the
// member they name. It deletes the list whole when no other item is left.
func (pl *planner) list(group []proseBlock) {
	if pl.covered[group[0].first] {
		return
	}
	names := make([]int, len(group))
	total, entries := 0, 0
	for n, b := range group {
		names[n] = pl.itemNames(b)
		if names[n] > 0 {
			total += names[n]
			entries++
		}
	}
	if total >= MinMembers && entries == len(group) {
		pl.hit(group[0].first, group[0].text)
		pl.cover(group[0].first, groupEnd(pl.lines, group))
		pl.intro(group[0].first)
		return
	}
	keys := make([]string, len(group))
	for n, b := range group {
		if names[n] > 0 && total >= MinMembers {
			pl.hit(b.first, b.text)
			pl.cover(b.first, groupEnd(pl.lines, []proseBlock{b}))
			continue
		}
		keys[n], _ = FrameKey(b.text)
	}
	for _, run := range SameKeyRuns(keys) {
		pl.hit(group[run[0]].first, group[run[0]].text)
		for _, b := range group[run[0]:run[1]] {
			pl.cover(b.first, groupEnd(pl.lines, []proseBlock{b}))
		}
	}
}

// itemNames answers the members an item names when the item, its nested
// lines included, only names things.
func (pl *planner) itemNames(b proseBlock) int {
	n := NamesOnly(b.text)
	if n == 0 {
		return 0
	}
	for i := b.last + 1; i <= groupEnd(pl.lines, []proseBlock{b}); i++ {
		if strings.TrimSpace(pl.lines[i]) == "" {
			continue
		}
		inner := pl.blockAt(i)
		if inner == nil {
			return 0
		}
		if inner.first != i {
			continue
		}
		m := NamesOnly(inner.text)
		if m == 0 {
			return 0
		}
		n += m
	}
	return n
}

// blockAt answers the prose block that holds line i.
func (pl *planner) blockAt(i int) *proseBlock {
	for k := range pl.blocks {
		if pl.blocks[k].first <= i && i <= pl.blocks[k].last {
			return &pl.blocks[k]
		}
	}
	return nil
}

// prose takes the enumerations out of a paragraph or an item that stays. A
// paragraph of nothing else goes whole. An item of nothing else goes whole
// only when each of its sentences is a bare list behind a set frame. It has
// no nested lines. An item that states something keeps a word.
func (pl *planner) prose(b proseBlock) {
	if pl.covered[b.first] {
		return
	}
	text := pl.content[b.start:b.end]
	cuts := ProseCuts(text)
	if len(cuts) == 0 {
		return
	}
	if b.item && !keepsWords(text, spansOf(cuts)) {
		if len(cuts) < len(SentenceSpans(text)) || groupEnd(pl.lines, []proseBlock{b}) > b.last {
			return
		}
		for _, c := range cuts {
			if !FramedList(text[c.Start:c.End]) {
				return
			}
		}
	}
	if len(cuts) == len(SentenceSpans(text)) {
		for _, c := range cuts {
			pl.hit(lineAt(pl.starts, b.start+c.Start), c.Detail)
		}
		pl.cover(b.first, b.last)
		return
	}
	for _, c := range cuts {
		pl.hit(lineAt(pl.starts, b.start+c.Start), c.Detail)
		pl.cuts = append(pl.cuts, Span{b.start + c.Start, b.start + c.End})
	}
}
