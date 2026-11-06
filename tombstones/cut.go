// cutReferents cuts the sentence around each dead name a whole-line strip left, and notes every name it could not cut.

package tombstones

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
)

// cutReferents cuts the sentence around each dead name a whole-line strip left,
// and notes every name it could not cut.
func cutReferents(f *fixer.File, path string) {
	text := f.Text()
	blocks := AddedBlocks(path, text)
	lines := strings.Split(text, "\n")
	starts := lineStarts(lines)
	var edits []edit.Edit
	// One edit per block, so no edits overlap. The next pass of the fixer cuts a second name.
	cutBlocks := set.New[int]()
	for _, name := range DeadReferents(path, text, blocks) {
		// HitForName counts its line from one, and lines counts from zero.
		lineNo := HitForName(blocks, name).LineNo - 1
		if lineNo < 0 || lineNo >= len(lines) {
			continue
		}
		if i := blockHolding(blocks, lineNo); i >= 0 && !IsDocument(path) {
			if cutBlocks.Contains(i) {
				continue
			}
			// A sentence that runs over several lines goes whole, and the lines around it stay.
			if from, to, ok := pureSpan(blocks[i], len(lines)); ok {
				if short, cut := commentfix.CutSentence(lines[from:to+1], name); cut {
					cutBlocks.Add(i)
					e := edit.Lines(text, from, to, 0, short)
					e.Cut = []string{name}
					edits = append(edits, e)
					continue
				}
			}
			cutBlocks.Add(i)
		}
		line := lines[lineNo]
		// A document line is prose end to end. A code line's prose bounds the cut, so the code on it stays.
		openers, prose, ok := []string(nil), line, true
		if !IsDocument(path) {
			openers = commentOpeners
			prose, ok = proseLine(blocks, lineNo)
		}
		if !ok || len(prose) != len(line) {
			continue
		}
		if from, to, ok := sentenceAround(line, prose, name, openers); ok {
			at := starts[lineNo]
			edits = append(edits, edit.Edit{Start: at + from, End: at + to, Cut: []string{strings.TrimSpace(line[from:to])}})
		}
	}
	f.ApplyComments(edits)
	after := AddedBlocks(path, f.Text())
	for _, name := range DeadReferents(path, f.Text(), after) {
		f.Note(HitForName(after, name))
	}
}

// proseLine answers the comment prose of line lineNo: the line with every byte
// outside a comment blanked.
func proseLine(blocks []Block, lineNo int) (string, bool) {
	for _, b := range blocks {
		prose := strings.Split(b.Prose, "\n")
		for i, no := range b.LineNos {
			if no == lineNo && i < len(prose) {
				return prose[i], true
			}
		}
	}
	return "", false
}

// blockHolding answers the index of the block that holds line lineNo, or a
// negative index when no block does.
func blockHolding(blocks []Block, lineNo int) int {
	for i, b := range blocks {
		if slices.Contains(b.LineNos, lineNo) {
			return i
		}
	}
	return -1
}

// commentOpeners open the prose of a comment. The longest one at the comment's start is its marker.
var commentOpeners = []string{"///", "//!", "//", "/**", "/*", "#", "--", "* "}

// sentenceAround answers the byte span of the sentence in line that holds name,
// with the blank in front of it. prose is line with every byte outside a
// comment blanked, so the parse that found the comment also bounds the cut. The
// code on the line stays.
func sentenceAround(line, prose, name string, openers []string) (int, int, bool) {
	at := strings.LastIndex(prose, name)
	start := len(prose) - len(strings.TrimLeft(prose, " \t"))
	end := len(strings.TrimRight(prose, " \t"))
	if at < 0 || at < start {
		return 0, 0, false
	}
	floor, marked := start, false
	for _, opener := range openers {
		if strings.HasPrefix(line[start:], opener) && start+len(opener) > floor {
			floor, marked = start+len(opener), true
		}
	}
	closer := end
	if strings.HasSuffix(line[:end], "*/") && end-len("*/") >= at+len(name) {
		closer = end - len("*/")
	}
	from := floor
	if i := strings.LastIndex(line[floor:at], ". "); i >= 0 {
		from = floor + i + 1
	}
	to := closer
	if i := strings.Index(line[at:closer], ". "); i >= 0 {
		to = at + i + 1
	}
	for to < closer && line[to] == '.' {
		to++
	}
	// A block comment that runs on past this line keeps its opener.
	unclosed := strings.HasPrefix(line[start:], "/*") && closer == end
	whole := from == floor && marked && !unclosed && strings.TrimSpace(line[to:closer]) == ""
	switch {
	case whole && strings.TrimSpace(line[end:]) == "":
		// The sentence is the whole comment and nothing follows it, so the comment goes with the blanks before it.
		from, to = len(strings.TrimRight(line[:start], " \t")), len(line)
	case whole:
		// The sentence is the whole comment and code follows it, so the comment goes with one run of blanks beside it.
		from, to = start, end
		if to < len(line) && line[to] == ' ' {
			for to < len(line) && line[to] == ' ' {
				to++
			}
		} else {
			for from > 0 && line[from-1] == ' ' {
				from--
			}
		}
	case from > floor:
		// The period that ends the sentence before stays, and the blank after it goes.
		for from < at && line[from] == ' ' {
			from++
		}
		from--
	default:
		// The blank after the marker stays, and the blanks after the sentence go.
		for from < at && line[from] == ' ' {
			from++
		}
		for to < len(line) && line[to] == ' ' {
			to++
		}
	}
	return from, to, from < to
}
