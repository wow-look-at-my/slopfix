// opening.go keeps a comment's first sentence when a cut takes the rest.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// layout lays a sentence out as comment lines, and reports whether they fit.
type layout func(sentence, indent, marker string) ([]string, bool)

// steOpening answers a block's first sentence, repaired to STE and divided at a
// clause boundary when it runs past the cap. A first sentence that fit does not
// accept divides again at a lower cap each time, and its own first sentence
// replaces it. It reports false when no division gives a whole sentence.
func steOpening(text []string, fit layout) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return nil, false
	}
	for _, para := range paragraphs(text) {
		if para.blank || para.verbatim {
			continue
		}
		// The opening sentence is the comment's own words.
		sentences := ste.Sentences(ste.FixKeepingOpening(closeThoughts(para.lines)))
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
		for limit := ste.WordCount(first) - 1; !fits && limit >= minimumOpening; limit-- {
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
		return out, true
	}
	return nil, false
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
