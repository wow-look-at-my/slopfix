// opening.go keeps a comment's first sentence when a cut takes the rest.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
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
		// A heading such as "# Safety" is not the comment's sentence. Keeping it
		// as the opening would delete the claim under it.
		if !endsSentence(first) || isHeading(first) {
			return nil, false
		}
		out, fits := fit(first, indent, marker)
		for limit := ste.WordCount(first) - 1; shrink && !fits && limit >= minimumOpening; limit-- {
			divided := ste.Sentences(ste.DivideTo(first, limit))
			if len(divided) < 2 {
				continue
			}
			head := strings.TrimSpace(divided[0])
			if !endsSentence(head) || isHeading(head) {
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
	joined := closeThoughts(lines)
	for limit := ste.SentenceWordCap; limit >= minimumOpening; limit-- {
		head, ok := ste.PhraseHead(joined, limit)
		if !ok {
			return nil, false
		}
		if out, fits := fit(head, indent, marker); fits {
			return out, true
		}
		limit = min(limit, ste.WordCount(head))
	}
	return nil, false
}

// minimumOpening is the fewest words a divided opening sentence keeps.
const minimumOpening = 3

// isHeading reports a markdown section heading such as "# Safety", which is
// structure rather than a sentence.
func isHeading(s string) bool {
	t := strings.TrimSpace(s)
	return t == "#" || strings.HasPrefix(t, "# ")
}

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
