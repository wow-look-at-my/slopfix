package tombstones

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
	"github.com/wow-look-at-my/slopfix/english"
)

// DefaultMaxCommentLines caps a comment block, the tier no rewording defeats.
const DefaultMaxCommentLines = 14

// Repair is the text with every strippable tombstone line deleted and every
// over-long comment reflowed. Kept carries what survives both, and a caller
// refuses on those.
type Repair struct {
	Text     string   `json:"text"`
	Changed  bool     `json:"changed"`
	Removed  []string `json:"removed,omitempty"`
	Rewrites int      `json:"rewrites,omitempty"`
	Kept     []Hit    `json:"kept,omitempty"`
}

// rewriteComments applies the english table to every comment block and reflows
// what it leaves. It answers the new text and how many rewrites that took.
//
// Every block goes through it, not only the ones over the cap: a tombstone is a
// so an earlier splice never moves a later block's line numbers.
func rewriteComments(added string, blocks []Block) (string, int) {
	lines := strings.Split(added, "\n")
	rewrites := 0
	changed := false
	for i := len(blocks) - 1; i >= 0; i-- {
		from, to, ok := blockSpan(blocks[i], len(lines))
		if !ok {
			continue
		}
		short, took, rewrote := commentlength.Tighten(lines[from : to+1])
		if !rewrote {
			continue
		}
		out := make([]string, 0, len(lines)-(to-from+1)+len(short))
		out = append(out, lines[:from]...)
		out = append(out, short...)
		out = append(out, lines[to+1:]...)
		lines = out
		rewrites += took
		changed = true
	}
	if !changed {
		return added, 0
	}
	return strings.Join(lines, "\n"), rewrites
}

// rewriteParagraphs is rewriteComments for a document, where a block is a
// paragraph rather than a comment. A paragraph carries no marker and is never
// hard-wrapped, so it goes back as the single line the table leaves.
func rewriteParagraphs(added string, blocks []Block) (string, int) {
	lines := strings.Split(added, "\n")
	rewrites := 0
	for i := len(blocks) - 1; i >= 0; i-- {
		from, to, ok := blockSpan(blocks[i], len(lines))
		if !ok {
			continue
		}
		body := strings.Join(lines[from:to+1], " ")
		// A backtick span is a literal rather than a claim, and the table reads
		if strings.Contains(body, "`") {
			continue
		}
		short, took := english.FixN(body, english.Comment)
		if took == 0 {
			continue
		}
		out := make([]string, 0, len(lines)-(to-from))
		out = append(out, lines[:from]...)
		if hasWord(short) {
			out = append(out, short)
		}
		out = append(out, lines[to+1:]...)
		lines = out
		rewrites += took
	}
	if rewrites == 0 {
		return added, 0
	}
	return strings.Join(lines, "\n"), rewrites
}

// hasWord reports whether any letter or digit survives, so a paragraph the
// table emptied is recognised as gone.
func hasWord(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// unbroken run is left alone, because splicing it would move code.
func blockSpan(b Block, total int) (from, to int, ok bool) {
	if len(b.LineNos) == 0 {
		return 0, 0, false
	}
	from, to = b.LineNos[0], b.LineNos[len(b.LineNos)-1]
	if from < 0 || to >= total || to-from+1 != len(b.LineNos) {
		return 0, 0, false
	}
	return from, to, true
}

// blocksLosing names the blocks a strip takes a line out of, by index.
func blocksLosing(blocks []Block, drop set.Set[int]) set.Set[int] {
	losing := set.New[int]()
	for i, b := range blocks {
		for _, no := range b.LineNos {
			if drop.Contains(no) {
				losing.Add(i)
				break
			}
		}
	}
	return losing
}

// reflowStripped rewraps each block a strip took a line out of, so the prose
// that survives reads as a paragraph rather than as a sentence with a hole.
//
// moves every index after it. That case is left alone rather than guessed at.
func reflowStripped(path, text string, losing set.Set[int], was int) string {
	if losing.IsEmpty() {
		return text
	}
	lines := strings.Split(text, "\n")
	blocks := AddedBlocks(path, text)
	if len(blocks) != was {
		return text
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		if !losing.Contains(i) {
			continue
		}
		from, to, ok := blockSpan(blocks[i], len(lines))
		if !ok {
			continue
		}
		short, _, rewrapped := commentlength.Tighten(lines[from : to+1])
		if !rewrapped {
			continue
		}
		out := make([]string, 0, len(lines)-(to-from+1)+len(short))
		out = append(out, lines[:from]...)
		out = append(out, short...)
		out = append(out, lines[to+1:]...)
		lines = out
	}
	return strings.Join(lines, "\n")
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

	// A block over the cap is rewritten before it is judged.
	was := added
	rewrite := rewriteComments
	if doc {
		rewrite = rewriteParagraphs
	}
	added, rewrites := rewrite(added, blocks)
	if added != was {
		blocks = AddedBlocks(path, added)
	}

	hits := Find(blocks, maxLines)
	for _, name := range DeadReferents(path, added, blocks) {
		hits = append(hits, HitForName(blocks, name))
	}
	if len(hits) == 0 {
		return Repair{Text: added, Changed: added != was, Rewrites: rewrites}
	}
	if doc {
		// A document line is a paragraph rather than a sentence, so
		// deleting it takes a keeper with it.
		for i := range hits {
			hits[i].Strippable = false
		}
	}

	repair := Repair{Text: added, Changed: added != was, Rewrites: rewrites}
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
	// The strip leaves a paragraph with a hole in it, so what survives is
	// rewrapped here. The write then lands finished, rather than landing broken
	// with a note asking for it to be read back and repaired.
	repair.Text = reflowStripped(path, strings.Join(kept, "\n"), blocksLosing(blocks, drop), len(blocks))
	repair.Changed = repair.Changed || repair.Text != added
	return repair
}
