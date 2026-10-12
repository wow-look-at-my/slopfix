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
		h := HitForName(blocks, name)
		if h.LineNo < 0 || h.LineNo >= len(lines) {
			continue
		}
		if i := blockHolding(blocks, h.LineNo); i >= 0 && !IsDocument(path) {
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
		openers := commentOpeners
		if IsDocument(path) {
			openers = nil
		}
		if from, to, ok := sentenceAround(lines[h.LineNo], name, openers); ok {
			at := starts[h.LineNo]
			edits = append(edits, edit.Edit{Start: at + from, End: at + to, Cut: []string{strings.TrimSpace(lines[h.LineNo][from:to])}})
		}
	}
	f.ApplyComments(edits)
	after := AddedBlocks(path, f.Text())
	for _, name := range DeadReferents(path, f.Text(), after) {
		f.Note(onLine(HitForName(after, name)))
	}
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

// onLine answers a dead-name hit as a finding reports it: HitForName counts its
// line from zero, and a finding counts lines from one.
func onLine(h Hit) Hit {
	if h.LineNo >= 0 {
		h.LineNo++
	}
	return h
}

// commentOpeners start the prose of a line. A sentence never reaches back past one.
var commentOpeners = []string{"//", "/*", "#", "--", "* "}

// sentenceAround answers the byte span of the sentence in line that holds
// name, with the blank in front of it. The span never reaches back past a
// comment marker, so the code on the line stays.
func sentenceAround(line, name string, openers []string) (int, int, bool) {
	at := strings.LastIndex(line, name)
	if at < 0 {
		return 0, 0, false
	}
	floor, marker := 0, 0
	for _, opener := range openers {
		if i := strings.LastIndex(line[:at], opener); i >= 0 && i+len(opener) > floor {
			floor, marker = i+len(opener), i
		}
	}
	from := floor
	if i := strings.LastIndex(line[floor:at], ". "); i >= 0 {
		from = floor + i + 1
	}
	to := len(line)
	if i := strings.Index(line[at:], ". "); i >= 0 {
		to = at + i + 1
	} else if i := strings.Index(line[at:], "*/"); i >= 0 {
		to = at + i
	}
	for to < len(line) && line[to] == '.' {
		to++
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[to:]), "*/"))
	switch {
	case from > floor:
		// The period that ends the sentence before stays, and the blank after it goes.
		for from < at && line[from] == ' ' {
			from++
		}
		from--
	case rest == "" && floor > 0:
		// The sentence is the whole comment, so the comment goes, marker and all.
		from, to = len(strings.TrimRight(line[:marker], " \t")), len(line)
	default:
		for to < len(line) && line[to] == ' ' {
			to++
		}
	}
	return from, to, from < to
}
