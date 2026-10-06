package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// punctuationDivision divides at a colon or at the dash that closes an aside,
// when no clause boundary reads the sentence. Each mark ends a thought the
// sentence already stated. So the half before it stands and the half after it
// opens as a sentence of its own. This is the last division before the words
// themselves are cut apart.
func punctuationDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	best, bestScore := "", -1
	for _, at := range punctuationCuts(masked) {
		left, right := source[:at], source[at:]
		head := closeHead(strings.TrimRight(left, " "))
		rest := capitalizeOpening(strings.TrimLeft(right, " "))
		if !StandsAlone(rest) {
			// "This is" carries a noun phrase that a dash introduces.
			if carried := "This is " + strings.TrimLeft(right, " "); StandsAlone(capitalizeOpening(carried)) {
				rest = capitalizeOpening(carried)
			}
		}
		if WordCount(head) < minimumHalf || WordCount(rest) < minimumHalf || !holdsFinite(checkMask(head)) || !StandsAlone(rest) {
			continue
		}
		if w, ok := lastWordBefore(whole, len(left)); ok && danglingTags.Contains(w.Tag) {
			continue
		}
		if !divides(head, rest, limit) {
			continue
		}
		if score := max(WordCount(head), WordCount(rest)); best == "" || score < bestScore {
			best, bestScore = head+" "+rest, score
		}
	}
	return best, bestScore >= 0
}

// punctuationCuts answers where a colon or a closing aside dash ends a clause
// the words before it hold. A mark inside a code span, a link, a quotation or a
// parenthesis is data, and ends nothing.
func punctuationCuts(masked string) []int {
	off := verbatimSpan.FindAllStringIndex(masked, -1)
	off = append(off, quotedSpans(masked)...)
	off = append(off, parenthetical.FindAllStringIndex(masked, -1)...)
	var out []int
	depth, code := 0, false
	for i := 0; i < len(masked); i++ {
		switch masked[i] {
		case '`':
			code = !code
		case '(', '[':
			depth++
		case ')', ']':
			depth = max(depth-1, 0)
		case ':':
			if depth == 0 && !code && !insideAny(off, i) {
				out = append(out, i+1)
			}
		}
	}
	spans := asides(masked)
	for _, a := range spans {
		out = append(out, a[1])
	}
	for _, d := range asideDash.FindAllStringIndex(masked, -1) {
		if insideAny(spans, d[0]) {
			continue
		}
		out = append(out, d[1])
	}
	return out
}
