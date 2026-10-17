package ste

import (
	"strings"
	"unicode"
)

// finalWord answers the last blank-separated field of sentence in lower case,
// without the opening marks before it. "(e.g." is the abbreviation behind an
// opening mark. It reads back from the end, so a long sentence costs no more
// than a short one.
func finalWord(sentence []rune) string {
	end := len(sentence)
	for end > 0 && unicode.IsSpace(sentence[end-1]) {
		end--
	}
	from := end
	for from > 0 && !unicode.IsSpace(sentence[from-1]) {
		from--
	}
	if from == end {
		return ""
	}
	return strings.ToLower(strings.TrimLeft(string(sentence[from:end]), "([\"'“‘*_"))
}

// markTally holds the open and closed parentheses of a sentence, and which
// opening marks it holds. It extends to a later end without reading the
// sentence again.
type markTally struct {
	from, to    int
	open, shut  int
	held        [6]bool
	initialized bool
}

// tallyMarks are the opening marks a tally records, in held's order.
var tallyMarks = [6]rune{'“', '‘', '*', '_', '"', '\''}

// span makes the tally cover runes[from:to].
func (t *markTally) span(runes []rune, from, to int) {
	if !t.initialized || from != t.from || to < t.to {
		*t = markTally{from: from, to: from, initialized: true}
	}
	for ; t.to < to; t.to++ {
		switch r := runes[t.to]; r {
		case '(':
			t.open++
		case ')':
			t.shut++
		default:
			for k, m := range tallyMarks {
				if r == m {
					t.held[k] = true
				}
			}
		}
	}
}

// closes reports a mark after a terminator that closes what the sentence so
// far opened. A `*` with no emphasis open before it is the glob of
// `esnext.*`, and a `)` with no open parenthesis is a word.
func (t *markTally) closes(mark rune) bool {
	switch mark {
	case ')':
		return t.open > t.shut
	case '”':
		return t.held[0]
	case '’':
		return t.held[1]
	case '*', '_', '"', '\'':
		for k, m := range tallyMarks {
			if m == mark {
				return t.held[k]
			}
		}
	}
	return false
}
