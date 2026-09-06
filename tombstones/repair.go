package tombstones

import (
	"github.com/wow-look-at-my/go-containers/set"
	"strings"
)

// DefaultMaxCommentLines caps one comment block in source. Volume is the tier
// no rewording defeats: an essay whose every sentence reads as true and current
// still fails here.
const DefaultMaxCommentLines = 14

// Repair is what a caller acts on: the text with every strippable tombstone
// line deleted, and the findings that no deletion can resolve.
type Repair struct {
	Text    string   `json:"text"`
	Changed bool     `json:"changed"`
	Removed []string `json:"removed,omitempty"`
	// Kept carries the findings that survive the strip. A caller that refuses a
	// write refuses on these, because a strip that guesses at a span corrupts
	// the file worse than a round trip back to the author does.
	Kept []Hit `json:"kept,omitempty"`
}

// Fix scans the text a write adds to path and strips what it safely can.
//
// maxLines caps one comment block; pass DefaultMaxCommentLines for the usual
// bound and zero to turn the cap off. A document is always capless, because a
// long paragraph is ordinary writing.
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
		// A document line is a paragraph, not a sentence: this org writes
		// without hard wraps, so several sentences share one raw line and
		// deleting the line takes a keeper with it.
		for i := range hits {
			hits[i].Strippable = false
		}
	}

	repair := Repair{Text: added}
	drop := set.New[int]()
	for _, h := range hits {
		if h.Strippable {
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
