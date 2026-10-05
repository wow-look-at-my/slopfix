// opening.go keeps a comment's first sentence when a cut takes the rest.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// layout lays a sentence out as comment lines, and reports whether they fit.
type layout func(sentence, indent, marker string) ([]string, bool)

// steOpening answers a block's first sentence, repaired to STE and divided at a
// clause boundary when it runs past the cap. With shrink, a first sentence that
// fit does not accept divides again at a lower cap each time, and its own first
// sentence replaces it. It reports false when no division gives a whole sentence.
func steOpening(text []string, fit layout, shrink bool) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return nil, false
	}
	for _, para := range paragraphs(text) {
		if para.blank || para.verbatim {
			continue
		}
		// The opening sentence is the comment's own words. A last thought with no stop gets one, so the division can close it.
		joined := closeThoughts(para.lines)
		if !endsSentence(joined) {
			joined += "."
		}
		sentences := ste.Sentences(ste.FixKeepingOpening(joined))
		if len(sentences) == 0 {
			return nil, false
		}
		first := strings.TrimSpace(sentences[0])
		if ste.WordCount(first) > ste.SentenceWordCap {
			if clause, ok := ste.Leading(first); ok {
				first = strings.TrimSpace(ste.FixKeepingOpening(clause))
			}
		}
		if !endsSentence(first) {
			return nil, false
		}
		out, fits := fit(first, indent, marker)
		for limit := ste.WordCount(first) - 1; shrink && !fits && limit >= minimumOpening; limit-- {
			divided := ste.Sentences(ste.DivideTo(first, limit))
			if len(divided) < 2 {
				continue
			}
			head := strings.TrimSpace(divided[0])
			if !endsSentence(head) {
				continue
			}
			if shorter, ok := fit(head, indent, marker); ok {
				return shorter, true
			}
		}
		if shrink && !fits {
			if shorter, ok := phraseLines(para.lines, fit, indent, marker); ok {
				return shorter, true
			}
		}
		return out, true
	}
	return nil, false
}

// phraseLines keeps the longest run of leading lines that ends a noun phrase
// where the next line opens one. It serves prose that never closes, which no
// division reads as a sentence.
func phraseLines(lines []string, fit layout, indent, marker string) ([]string, bool) {
	for k := len(lines) - 1; k > 0; k-- {
		if !phraseBreak(lines[k-1], lines[k]) {
			continue
		}
		head := closeThoughts(lines[:k])
		if ste.WordCount(head) > ste.SentenceWordCap {
			continue
		}
		if !endsSentence(head) {
			head += "."
		}
		if out, ok := fit(head, indent, marker); ok {
			return out, true
		}
	}
	return nil, false
}

// phraseBreak reports a line that ends on a noun, an adverb or a number, before
// a line that opens a new noun phrase on its determiner.
func phraseBreak(prev, next string) bool {
	next = strings.TrimSpace(next)
	first, _, _ := strings.Cut(next, " ")
	switch first {
	case "a", "an", "the", "every", "each", "any", "some", "no":
	default:
		return false
	}
	s := syntax.Parse(strings.TrimSpace(prev), nil)
	if len(s.Words) == 0 {
		return false
	}
	last := s.Words[len(s.Words)-1]
	if dangling.Contains(last.Lower()) {
		return false
	}
	tag := last.Tag
	return strings.HasPrefix(tag, "NN") || strings.HasPrefix(tag, "RB") || tag == "CD"
}

// minimumOpening is the fewest words a divided opening sentence keeps.
const minimumOpening = 3

// closeThoughts joins prose lines, and writes a period where a line ends a
// thought with no stop of its own (lineEndsThought).
func closeThoughts(lines []string) string {
	var b strings.Builder
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i > 0 {
			if lineEndsThought(lines[i-1], line) {
				b.WriteString(".")
			}
			b.WriteString(" ")
		}
		b.WriteString(line)
	}
	return b.String()
}
