// force.go divides a long sentence that has no clause boundary.

package ste

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// force.go divides a long sentence that has no clause boundary. The division
// lands between words, near the cap, where each part is a sentence.

var (
	// forceDangling words never end the first part of a forced division.
	forceDangling = set.Of(wordsOf("force-dangling")...)
	// forceBound words never open the second part.
	forceBound = set.Of(wordsOf("force-bound")...)
	// forceOpener words open a phrase, so a division in front of one reads better.
	forceOpener = set.Of(wordsOf("force-opener")...)
	// linkText is the bracketed text of a markdown link, which no division halves.
	linkText = regexp.MustCompile(`\[[^\]]*\]`)
	// gapRun is the blank between a couple of words.
	gapRun = regexp.MustCompile(`\s+`)
	// firstToken is the word that opens a part.
	firstToken = regexp.MustCompile(`^\S+`)
)

// overCap counts the words past limit in every sentence Check reads in prose.
func overCap(prose string, limit int) int {
	n := 0
	for _, sentence := range Sentences(checkMask(prose)) {
		n += max(0, WordCount(sentence)-limit)
	}
	return n
}

// Masked is the prose as Check reads it, with every offset kept.
func Masked(prose string) string { return checkMask(prose) }

// checkMask writes the word Check's strip writes over each span it strips,
// byte for byte. The sentences, the word counts and the tags then match Check,
// and an offset into the mask is an offset into the prose.
func checkMask(prose string) string {
	out := []byte(prose)
	for _, span := range verbatimSpan.FindAllStringIndex(prose, -1) {
		switch prose[span[0]] {
		case '`':
			fillCheckWord(out[span[0]:span[1]], "CODE")
		case '&':
			fillCheckWord(out[span[0]:span[1]], "ENTITY")
		default:
			fillLink(out[span[0]:span[1]], func(inner []byte) { fillCheckWord(inner, "URL") })
		}
	}
	// Check finds a quotation after it strips code, so a quote mark inside a code span pairs with nothing.
	for _, span := range quotation.FindAllIndex(out, -1) {
		fillCheckWord(out[span[0]:span[1]], "QUOTE")
	}
	return string(out)
}

// fillCheckWord writes word over a span as fillWith does. A span too short to
// hold it gets the capitalized filler.
func fillCheckWord(span []byte, word string) {
	if len(span) < len(word)+2 {
		fillWord(span)
		return
	}
	fillWith(span, word)
}

// quotedSpans answers the quotations Check reads in prose, after code is masked.
func quotedSpans(prose string) [][]int {
	return quotation.FindAllStringIndex(mask(prose), -1)
}

// fillWord writes a capitalized filler word over a span, as Check writes CODE
// or QUOTE. A space at each end keeps the words beside it apart.
func fillWord(span []byte) {
	for i := range span {
		span[i] = 'X'
	}
	if len(span) > 2 {
		span[0], span[len(span)-1] = ' ', ' '
	}
}

// forceCut is a place between a couple of words where a forced division can land.
type forceCut struct {
	left, right int
	score       int
	// words is the count Check reads for the part before left.
	words int
	// afterNoun is a cut right after a noun, and bare a cut with no mark and no opener word.
	afterNoun, bare bool
}

// forceDivision divides source at the best word boundary that leaves the first
// part under the cap. It never divides inside a code span, a link, a quotation,
// a parenthesis or bold text.
func forceDivision(source, masked string, d capSpec) (string, bool) {
	// The tags come from the whole sentence, because a fragment parsed alone reads "a faithful" as a noun.
	whole := syntax.Parse(masked, nil)
	if out, ok := listDivision(source, masked, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	dashed := dashAsides(source)
	for _, strict := range []bool{true, false} {
		best, bestScore := "", 0
		for _, c := range candidates(source, masked, strict, d.cap) {
			if cutsAside(masked, c.left, c.right) || splitsObject(whole, c) {
				continue
			}
			head := source[:c.left]
			right, opened := openRest(source, masked, whole, c)
			// A rest that only gains a capital leaves the dash aside halved.
			if insideAny(dashed, c.left) && right == capitalizeOpening(strings.TrimLeft(source[c.right:], " —–-")) {
				continue
			}
			left := closeHead(head)
			seam := seamBefore(source, c.left)
			// ", so" joins whole clauses, so the comma before it ends one wherever it sits.
			if seam == "," && strings.HasPrefix(strings.ToLower(strings.TrimLeft(source[c.right:], " ")), "so ") {
				seam = "so"
			}
			// A fragment head behind "This is" closes as a sentence of its own.
			accept := func(h, r string) bool {
				joined := closeHead(h) + " " + r
				return r != "" && divides(closeHead(h), r, d.cap) &&
					(h == fragmentHead(source[:c.left]) || closesWhole(source[:c.left], seam, whole) || closesPhrase(source[:c.left], whole)) &&
					overCap(joined, d.cap) < overCap(source, d.cap)
			}
			if !accept(head, right) {
				// The grammatical rest was refused, so the carrier division, which restates the main clause, gets its turn.
				ch, cr, co := carrierOrNone(source, whole, c)
				cl := closeHead(ch)
				if cr == "" || !divides(cl, cr, d.cap) || overCap(cl+" "+cr, d.cap) >= overCap(source, d.cap) {
					continue
				}
				head, right, opened, left = ch, cr, co, cl
			}
			// A bare clause after a noun describes it: "the paths the file declared".
			if opened == opensOwnClause && c.afterNoun && c.bare {
				continue
			}
			// A rest that opens a clause of its own reads best.
			if score := c.score + opened; best == "" || score > bestScore {
				best, bestScore = left+" "+right, score
			}
		}
		// A division that leaves as many words over the cap moves nothing, and
		// forceSentenceCap would revert it.
		if best != "" && overCap(best, d.cap) < overCap(source, d.cap) {
			return best, true
		}
	}
	if d.reorder {
		if out, ok := reorderDependent(source, whole); ok && overCap(out, d.cap) < overCap(source, d.cap) {
			return out, true
		}
		if out, ok := subjectDivision(source, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
			return out, true
		}
	}
	if out, ok := fragmentDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := markDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := coordinateDivision(whole, source); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := becauseDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	// A pair of dashes is one aside. It divides whole, with the aside kept
	// behind a comma, before the punctuation cut reads the closing dash.
	if out, ok := asideDivision(source, masked, d.cap); ok {
		return out, true
	}
	if out, ok := punctuationDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := hardDivision(source, masked, d.cap); ok || !d.every {
		return out, ok
	}
	if out, ok := standingMarkDivision(source, masked, d.cap); ok {
		return out, true
	}
	return anyDivision(source, masked, d.cap)
}
