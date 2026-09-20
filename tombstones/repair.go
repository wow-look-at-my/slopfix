package tombstones

import (
	"github.com/wow-look-at-my/go-containers/set"
	"strings"
)

// DefaultMaxCommentLines caps a comment block, the tier no rewording defeats.
const DefaultMaxCommentLines = 14

// Repair is the text with every strippable tombstone line deleted. Kept carries
// what survives the strip, and a caller refuses on those.
type Repair struct {
	Text    string   `json:"text"`
	Changed bool     `json:"changed"`
	Removed []string `json:"removed,omitempty"`
	Kept    []Hit    `json:"kept,omitempty"`
}

// codeRow reports a comment line indented past its own marker. godoc, and
// every renderer that follows it, prints such a run verbatim as a code block.
// Its rows are a table rather than sentences, so a strip that takes one leaves
// the rows around it describing records nothing names. Three rows of exactly
// this shape lost two of them to the word "original".
func codeRow(path string, lines []string, n int) bool {
	if n < 0 || n >= len(lines) {
		return false
	}
	st, ok := styleFor(path)
	if !ok {
		return false
	}
	trimmed := strings.TrimLeft(lines[n], " \t")
	for _, marker := range st.line {
		rest, found := strings.CutPrefix(trimmed, marker)
		if !found {
			continue
		}
		return strings.HasPrefix(rest, "\t") || strings.HasPrefix(rest, "    ")
	}
	return false
}

// Fix scans the text a write adds to path and strips what it safely can.
// maxLines caps a comment block, and a cap below the floor turns it off.
func Fix(path, added string, maxLines int) Repair {
	doc := IsDocument(path)
	if doc {
		maxLines = 0
	}
	blocks := AddedBlocks(path, added)
	if len(blocks) == 0 {
		return Repair{Text: added}
	}

	hits := Find(blocks, maxLines)
	for _, name := range DeadReferents(path, added, blocks) {
		hits = append(hits, HitForName(blocks, name))
	}
	if len(hits) == 0 {
		return Repair{Text: added}
	}
	if doc {
		// A document line is a paragraph rather than a sentence, so
		// deleting it takes a keeper with it.
		for i := range hits {
			hits[i].Strippable = false
		}
	}

	repair := Repair{Text: added}
	lines := strings.Split(added, "\n")
	drop := set.New[int]()
	for _, h := range hits {
		if h.Strippable && !codeRow(path, lines, h.LineNo) {
			drop.Add(h.LineNo)
			continue
		}
		repair.Kept = append(repair.Kept, h)
	}
	if drop.Len() == 0 {
		return repair
	}

	seen := set.New[string]()
	var kept []string
	for i, line := range strings.Split(added, "\n") {
		if drop.Contains(i) {
			trimmed := strings.TrimSpace(line)
			if !seen.Contains(trimmed) {
				seen.Add(trimmed)
				repair.Removed = append(repair.Removed, trimmed)
			}
			continue
		}
		kept = append(kept, line)
	}
	repair.Text = strings.Join(kept, "\n")
	repair.Changed = repair.Text != added
	return repair
}
