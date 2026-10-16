package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// verblessPenalty puts every cut whose rest holds no verb below every cut whose rest holds one.
const verblessPenalty = 1 << 10

// verbless reports a rest after cut c that holds no verb of its own, such as
// "At a time." or "Up to the noun it counts". It reads as no sentence.
func verbless(whole *syntax.Sentence, c forceCut, source string) bool {
	rest := opening(hardRest(source, c.right))
	if strandsPhrase(whole, c.right, rest) || !standsAsSentence(rest) {
		return true
	}
	if holdsFinite(checkMask(rest)) {
		return false
	}
	// A capital turns a noun into a verb for the tagger, so a rest reads as an instruction only after an instruction.
	return !(opensImperative(checkMask(rest)) && opensImperativeMain(checkMask(source[:c.left])))
}

// carriedRest gives a verbless rest the subject and verb it lacks, so the
// division still ends in sentences. A phrase that opens on a preposition or
// an adverb says where or how the claim holds. Any other rest names what the
// claim covers.
func carriedRest(whole *syntax.Sentence, c forceCut, rest string) string {
	body := lowerFirst(rest)
	if n := wordFrom(whole, c.right); n >= 0 && n < len(whole.Words) {
		switch whole.Words[n].Tag {
		case "IN", "TO", "RB", "RP":
			return "This holds " + body
		}
	}
	return "This covers " + body
}

// lastResortDivision divides a sentence that every filter of hardDivision
// refused, so no path leaves it over the cap. It prefers the longest head
// under the cap that reads as a sentence. Then a head that ends on a word
// that closes a phrase, then the first words the cap allows outside any
// verbatim span. Each half then reads as a sentence: a head with no clause
// and a rest with no verb each take a carrier.
func lastResortDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	// A carrier adds its words to a half, so a carried head keeps room for them.
	room := limit - len(strings.Fields(carrierWords))
	var cuts []forceCut
	for _, c := range candidates(source, masked, false, limit) {
		if c.words <= limit && c.words >= minimumHalf && strings.TrimSpace(source[c.right:]) != "" && !endsOnPossessive(source, c.left) {
			cuts = append(cuts, c)
		}
	}
	pick := func(ok func(forceCut) bool) (forceCut, bool) {
		best, found := forceCut{}, false
		for _, c := range cuts {
			if ok(c) && (!found || c.words > best.words) {
				best, found = c, true
			}
		}
		return best, found
	}
	c, found := pick(func(c forceCut) bool {
		head := closeHead(source[:c.left])
		return !cutsAside(masked, c.left, c.right) && standsAsSentence(head) && headHoldsMainVerb(whole, c.left) && closesAPhrase(head) &&
			!forceBound.Contains(firstWordOf(source[c.right:]))
	})
	if !found {
		c, found = pick(func(c forceCut) bool {
			return c.words <= room && closesAPhrase(source[:c.left]) && !forceBound.Contains(firstWordOf(source[c.right:]))
		})
	}
	if !found {
		var ok bool
		if c, ok = capCut(source, masked, room); !ok {
			return source, false
		}
	}
	head := closeHead(source[:c.left])
	if !standsAsSentence(head) {
		head = carriedHead(head)
	}
	right := hardRest(source, c.right)
	if verbless(whole, c, source) {
		right = carriedRest(whole, c, right)
	}
	return head + " " + right, true
}

// headHoldsMainVerb reports a head that ends after the verb of a main clause
// of the whole sentence. The head carries a predicate of its own.
func headHoldsMainVerb(whole *syntax.Sentence, left int) bool {
	for _, clause := range whole.Clauses {
		if clause.Depth == 0 && clause.Verb != nil && whole.Words[clause.Verb.Last].End <= left {
			return true
		}
	}
	return false
}

// carrierWords are the words a carrier adds to the half it carries.
const carrierWords = "This covers"

// carriedHead gives a head with no clause of its own a subject and a verb: "This covers the list of the paths."
func carriedHead(head string) string {
	return carrierWords + " " + lowerFirst(head)
}

// closesAPhrase reports a head whose last word ends a phrase, not one that
// waits for the word after it, such as "the" or "of".
func closesAPhrase(head string) bool {
	last := strings.ToLower(strings.Trim(lastField(head), ".,;:!?*_\"'`()[]“”‘’"))
	return last != "" && !forceDangling.Contains(last) && !strings.HasSuffix(last, "'s")
}

// capCut answers a cut after the last word the cap allows that closes a
// phrase, outside any code span, link or entity.
func capCut(source, masked string, limit int) (forceCut, bool) {
	spans := dataSpans(source)
	var ends []int
	for _, end := range wordEnds(masked) {
		if !insideAny(spans, end-1) {
			ends = append(ends, end)
		}
	}
	if len(ends) < 2 {
		return forceCut{}, false
	}
	k := min(max(limit, 1), len(ends)-1)
	for i := k - 1; i >= minimumHalf-1; i-- {
		if closesAPhrase(source[:ends[i]]) {
			return forceCut{left: ends[i], right: ends[i]}, true
		}
	}
	return forceCut{left: ends[k-1], right: ends[k-1]}, true
}

// firstWordOf answers the first word of text in lower case, with no mark around it.
func firstWordOf(text string) string {
	return strings.ToLower(strings.Trim(firstToken.FindString(text), ".,;:!?*_\"'`()[]“”‘’"))
}
