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
	rest := hardRest(source, c.right)
	if strandsPhrase(whole, c.right, rest) || !standsAsSentence(rest) {
		return true
	}
	return !holdsFinite(checkMask(rest)) && !opensImperative(checkMask(rest))
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
// refused, so no path leaves it over the cap. It takes the longest head under
// the cap that reads as a sentence, then any head under the cap, then the
// words. Up to the cap. The rest takes a carrier when it holds no verb.
func lastResortDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	var cuts []forceCut
	for _, c := range candidates(source, masked, false, limit) {
		if c.words <= limit && c.words >= minimumHalf && strings.TrimSpace(source[c.right:]) != "" {
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
		return !cutsAside(masked, c.left, c.right) && standsAsSentence(closeHead(source[:c.left]))
	})
	if !found {
		c, found = pick(func(forceCut) bool { return true })
	}
	if !found {
		spans := verbatimSpan.FindAllStringIndex(source, -1)
		var ends []int
		for _, end := range wordEnds(masked) {
			if !insideAny(spans, end-1) {
				ends = append(ends, end)
			}
		}
		if len(ends) < 2 {
			return source, false
		}
		at := ends[min(limit, len(ends)-1)-1]
		c = forceCut{left: at, right: at}
	}
	right := hardRest(source, c.right)
	if verbless(whole, c, source) {
		right = carriedRest(whole, c, right)
	}
	return closeHead(source[:c.left]) + " " + right, true
}
