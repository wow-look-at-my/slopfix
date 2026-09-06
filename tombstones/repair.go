package tombstones

import (
	"github.com/wow-look-at-my/go-containers/set"
	"strings"
)

// DefaultMaxCommentLines caps a comment block, the tier no rewording defeats.
const DefaultMaxCommentLines = 14

// Repair is the text with every strippable tombstone line deleted.
//
// Kept carries the findings that survive the strip, and a caller refuses on
// those: a strip that guesses at a span corrupts the file.
type Repair struct {
	Text    string   `json:"text"`
	Changed bool     `json:"changed"`
	Removed []string `json:"removed,omitempty"`
	Kept    []Hit    `json:"kept,omitempty"`
}

// Fix scans the text a write adds to path and strips what it safely can.
//
// maxLines caps a comment block, and a cap below the floor turns it off. A
// document is always capless, because a long paragraph is ordinary writing.
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
