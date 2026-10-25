// repair_cuts.go cuts tombstone wording out of comment lines and caps a block at its line limit.

package tombstones

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// wordingHits answers a hit for each wording rule whose phrase the table would
// cut from the span e rewrites in text.
func wordingHits(text string, e edit.Edit, doc bool) []Hit {
	surface := english.Comment
	if doc {
		surface = english.Document
	}
	span := text[e.Start:e.End]
	lineNo := strings.Count(text[:e.Start], "\n") + 1
	first, _, _ := strings.Cut(span, "\n")
	ids := AllIDs()
	seen := set.New[string]()
	var hits []Hit
	for _, p := range english.Patterns() {
		if !ids.Contains(p.ID) || seen.Contains(p.ID) || !english.AppliesTo(p.Where, surface) {
			continue
		}
		if _, took := p.ApplyN(span); took == 0 {
			continue
		}
		seen.Add(p.ID)
		hits = append(hits, Hit{
			ID:     p.ID,
			Tell:   "a phrase about a state the code has left",
			Phrase: strings.TrimSpace(first),
			Line:   first,
			LineNo: lineNo,
		})
	}
	return hits
}

// cutTombstones applies every wording rule of this family to comment prose.
func cutTombstones(s string) (string, int) {
	ids := AllIDs()
	total := 0
	for _, p := range english.Patterns() {
		if !ids.Contains(p.ID) || !english.AppliesTo(p.Where, english.Comment) {
			continue
		}
		var took int
		s, took = p.ApplyN(s)
		total += took
	}
	return s, total
}

// commentLines calls visit with the line number and text of every comment line in
// blocks that is not shared with code and is no directive.
func commentLines(text string, blocks []Block, visit func(lineNo int, line string)) {
	lines := strings.Split(text, "\n")
	for _, b := range blocks {
		for i, lineNo := range b.LineNos {
			if i >= len(b.Pure) || !b.Pure[i] || lineNo < 0 || lineNo >= len(lines) || treecomments.DirectiveAt(lines, lineNo) {
				continue
			}
			visit(lineNo, lines[lineNo])
		}
	}
}

// phraseCuts answers an edit for each comment line that still holds a tombstone
// phrase, cutting that phrase and nothing else. How many cuts each took.
func phraseCuts(text string, blocks []Block) ([]edit.Edit, map[int]int) {
	took := map[int]int{}
	var edits []edit.Edit
	commentLines(text, blocks, func(lineNo int, line string) {
		short, n, ok := commentfix.RewriteLineProse(line, cutTombstones)
		if !ok {
			return
		}
		e := edit.Lines(text, lineNo, lineNo, 0, short)
		edits = append(edits, e)
		took[e.Start] = n
	})
	return edits, took
}

// phraseHits answers a hit on each comment line where a wording rule still finds
// its phrase.
func phraseHits(text string, blocks []Block) []Hit {
	ids := AllIDs()
	var hits []Hit
	commentLines(text, blocks, func(lineNo int, line string) {
		seen := set.New[string]()
		for _, p := range english.Patterns() {
			if !ids.Contains(p.ID) || seen.Contains(p.ID) || !english.AppliesTo(p.Where, english.Comment) {
				continue
			}
			if _, n, ok := commentfix.RewriteLineProse(line, p.ApplyN); !ok || n == 0 {
				continue
			}
			seen.Add(p.ID)
			hits = append(hits, Hit{
				ID:     p.ID,
				Tell:   "a phrase about a state the code has left",
				Phrase: strings.TrimSpace(line),
				Line:   line,
				LineNo: lineNo + 1,
			})
		}
	})
	return hits
}

// endsOnSentence reports whether the last line of block b that a cut keeps ends
// a sentence.
func endsOnSentence(lines []string, b Block, drop set.Set[int]) bool {
	for i := len(b.LineNos) - 1; i >= 0; i-- {
		no := b.LineNos[i]
		if drop.Contains(no) || no < 0 || no >= len(lines) {
			continue
		}
		text := strings.TrimRight(strings.TrimSpace(lines[no]), " */")
		return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?")
	}
	return false
}

// capEdits answers an edit per block over maxLines. A block of comment lines
// alone is cut at its thoughts. A block that shares a line with code loses its
// last prose lines instead, because a line edit there would carry the code.
func capEdits(text string, blocks []Block, maxLines int) []edit.Edit {
	if maxLines <= 0 {
		return nil
	}
	lines := strings.Split(text, "\n")
	var edits []edit.Edit
	for _, b := range blocks {
		if b.Lines <= maxLines {
			continue
		}
		if from, to, ok := pureSpan(b, len(lines)); ok {
			kept := commentfix.CapLines(lines[from:to+1], maxLines)
			edits = append(edits, edit.Lines(text, from, to, 0, kept))
			continue
		}
		drop := set.New[int]()
		over := b.Lines - maxLines
		for i := len(b.LineNos) - 1; i >= 0 && over > 0; i-- {
			no, pure := linePurity(b, i)
			if pure && no >= 0 && no < len(lines) && !treecomments.DirectiveAt(lines, no) {
				drop.Add(no)
				over--
			}
		}
		// A line cut that does not land on a sentence end leaves a fragment, so
		// the surviving line is trimmed back to one. The cap has to be
		// reachable: dropping further lines would cost more than the fragment.
		if !endsOnSentence(lines, b, drop) {
			if lineNo, trimmed, ok := trimLastKeptLine(lines, b, drop); ok {
				edits = append(edits, stripEdits(text, drop)...)
				edits = append(edits, edit.Lines(text, lineNo, lineNo, 0, []string{trimmed}))
				continue
			}
			// Lines go from the end until a kept line ends a sentence. A block with no such line keeps its prose.
			for !endsOnSentence(lines, b, drop) && dropLastKept(lines, b, drop) {
			}
			if !endsOnSentence(lines, b, drop) {
				continue
			}
		}
		edits = append(edits, stripEdits(text, drop)...)
	}
	return edits
}

// cuttable reports whether the cap cut has an edit for the block whose first
// line, counted from one, is lineNo. A block it cannot end on a sentence keeps
// its prose, so no finding names it.
func cuttable(text string, blocks []Block, maxLines, lineNo int) bool {
	for _, b := range blocks {
		if len(b.LineNos) > 0 && b.LineNos[0]+1 == lineNo {
			return len(capEdits(text, []Block{b}, maxLines)) > 0
		}
	}
	return false
}

// dropLastKept adds the last prose line a drop keeps to it, and reports false
// when no prose line is left to drop.
func dropLastKept(lines []string, b Block, drop set.Set[int]) bool {
	for i := len(b.LineNos) - 1; i >= 0; i-- {
		no, pure := linePurity(b, i)
		if pure && no >= 0 && no < len(lines) && !drop.Contains(no) && !treecomments.DirectiveAt(lines, no) {
			drop.Add(no)
			return true
		}
	}
	return false
}

// trimLastKeptLine cuts the last line a drop keeps back to its last sentence end.
// A volume cut lands on a sentence rather than a fragment. It reports false
// when that line holds no sentence end to cut back to.
func trimLastKeptLine(lines []string, b Block, drop set.Set[int]) (int, string, bool) {
	for i := len(b.LineNos) - 1; i >= 0; i-- {
		no, pure := linePurity(b, i)
		if !pure || drop.Contains(no) || no < 0 || no >= len(lines) {
			continue
		}
		if trimmed, ok := trimToSentenceEnd(lines[no]); ok {
			return no, trimmed, true
		}
		return 0, "", false
	}
	return 0, "", false
}

// trimToSentenceEnd removes whatever follows the last sentence end on a line,
// keeping a block closer when one follows it.
func trimToSentenceEnd(line string) (string, bool) {
	cut := strings.LastIndexAny(line, ".!?")
	if cut < 0 {
		return "", false
	}
	trimmed := line[:cut+1]
	if rest := line[cut+1:]; strings.Contains(rest, "*/") {
		trimmed += " */"
	}
	return trimmed, true
}

// cutLines quotes the dropped lines a strip edit covers.
func cutLines(lines []string, drop set.Set[int], e edit.Edit, text string) []string {
	var out []string
	lineNo := strings.Count(text[:max(e.Start, 0)], "\n")
	if e.Start > 0 && e.Start < len(text) && text[e.Start] == '\n' {
		// A strip of the last line starts on the line end before it.
		lineNo++
	}
	for ; lineNo < len(lines) && drop.Contains(lineNo); lineNo++ {
		out = append(out, strings.TrimSpace(lines[lineNo]))
	}
	return out
}
