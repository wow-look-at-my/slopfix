// force_hard.go divides a sentence no clause boundary fits, at the best seam a word boundary offers.

package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/syntax"
)

	ends := wordEnds(masked)
	if len(ends) <= limit || len(ends) < 2*minimumHalf {
		return source, false
	}
	whole := syntax.Parse(masked, nil)
	best, bestWords, bestRank := forceCut{}, -1, -1
	for _, c := range candidates(source, masked, false, limit) {
		if cutsAside(masked, c.left, c.right) || c.words > limit || c.words < minimumHalf {
			continue
		}
		if len(ends)-c.words < minimumHalf || dependentRest(whole, c) {
			continue
		}
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && danglingTags.Contains(w.Tag) && !predicateAdjective(whole, wordFrom(whole, w.Start)) {
			// A cut inside a noun phrase strands its noun: "no other | thread".
			continue
		}
		// A verb does not close a sentence before the adjective it links to:
		// "the pointer stays. Valid for reads of n bytes" is no sentence pair.
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && strings.HasPrefix(w.Tag, "VB") {
			if n := wordsBefore(whole, c.right); n < len(whole.Words) && strings.HasPrefix(whole.Words[n].Tag, "JJ") {
				continue
			}
		}
		// A tail that opens on a finite verb leaves its subject behind:
		// "the pointer. Stays valid for reads of n bytes".
		if n := wordsBefore(whole, c.right); n < len(whole.Words) && finiteVerbTag(whole.Words[n].Tag) {
			continue
		}
		// A relative word binds its clause to the noun in front of it: "the verb. That governs it".
		if n := wordsBefore(whole, c.right); n > 0 && n+1 < len(whole.Words) && relativeAfterNoun(whole, n) {
			continue
		}
		head := strings.TrimRight(source[:c.left], " ,;:—–-")
		// The head must hold a clause of its own, or the cut strands a subject
		// without its verb: "the pointer. Stays valid for reads of n bytes".
		if !holdsFinite(checkMask(head)) && !opensImperative(checkMask(head)) {
			continue
		}
		last := strings.ToLower(strings.Trim(lastField(head), ".,;:!?*_\"'`()[]“”‘’"))
		next := strings.ToLower(strings.Trim(firstToken.FindString(source[c.right:]), ".,;:!?*_\"'`()[]“”‘’"))
		if endsOnPossessive(source, c.left) {
			continue
		}
		if forceDangling.Contains(last) || forceBound.Contains(next) {
			continue
		}
		// Both parts must read as sentences, or the cut strands a fragment such
		// as "writes. Through it" -- and the start of the rest decides that.
		if !standsAsSentence(closeHead(source[:c.left])) ||
			!standsAsSentence(opening(hardRest(source, c.right))) {
			continue
		}
		// A rest that opens on a preposition and holds a finite verb only inside a relative clause has no main clause: "With the verb that governs it, laid out".
		if rs := syntax.Parse(opening(hardRest(source, c.right)), nil); len(rs.Words) > 0 && (rs.Words[0].Tag == "IN" || rs.Words[0].Tag == "TO") && !finiteOutsideRelative(rs, 0) {
			continue
		}
		// The best seam wins, then the longest leading run. The first cut of a tie is deterministic.
		rank := hardSeamRank(source, whole, c)
		if rank > bestRank || rank == bestRank && c.words > bestWords {
			best, bestWords, bestRank = c, c.words, rank
		}
	}
	if bestWords < 0 {
		best, bestWords = lastResortCut(source, masked, len(ends), limit)
	}
	if bestWords < 0 {
		return source, false
	}
	left := closeHead(source[:best.left])
	right := hardRest(source, best.right)
	if left == "" || right == "" {
		return source, false
	}
	return left + " " + right, true
}

// relativeAfterNoun reports a relative word at word n, such as "that" or
// "which", right behind a noun and in front of a verb. Its clause belongs to
// that noun, so no sentence opens on it.
func relativeAfterNoun(s *syntax.Sentence, n int) bool {
	switch s.Words[n].Lower() {
	case "that", "which", "who":
	default:
		return false
	}
	after := s.Words[n+1].Tag
	return strings.HasPrefix(s.Words[n-1].Tag, "NN") && (strings.HasPrefix(after, "VB") || after == "NNS")
}

// endsOnPossessive reports a cut whose head closes on a possessive. The
// possessive governs the word after it, so a part never ends on one.
func endsOnPossessive(source string, left int) bool {
	last := strings.ToLower(strings.Trim(lastField(strings.TrimRight(source[:left], " ,;:—–-")), ".,;:!?*_\"'`()[]“”‘’"))
	return strings.HasSuffix(last, "'s") || strings.HasSuffix(last, "’s")
}

// endsOnOpener reports a cut whose head ends on a word that needs the next
// one, such as an article, a preposition or a conjunction. The cut would part
// a phrase: "understanding the. Environment".
func endsOnOpener(source string, whole *syntax.Sentence, c forceCut) bool {
	last := strings.ToLower(strings.Trim(lastField(strings.TrimRight(source[:c.left], " ,;:—–-")), ".,;:!?*_\"'`()[]“”‘’"))
	if forceDangling.Contains(last) {
		return true
	}
	w, ok := lastWordBefore(whole, c.left)
	if ok && w.Tag == "VBG" {
		return true
	}
	return ok && danglingTags.Contains(w.Tag) && !predicateAdjective(whole, wordFrom(whole, w.Start))
}

// lastResortCut answers the cut that keeps the most leading words under the
// cap, for a sentence where no cut leaves both parts reading as sentences. A
// finding that fix leaves standing breaks the guarantee, and an awkward pair
// of sentences does not. The cut still lands between whole words, outside a
// code span, a link, a quotation and a parenthesis.
func lastResortCut(source, masked string, words, limit int) (forceCut, int) {
	best, bestWords, bestComma := forceCut{}, -1, false
	whole := syntax.Parse(masked, nil)
	for _, c := range candidates(source, masked, false, limit) {
		if cutsAside(masked, c.left, c.right) || c.words > limit || c.words < 1 || words-c.words < 1 {
			continue
		}
		if endsOnPossessive(source, c.left) || endsOnOpener(source, whole, c) {
			continue
		}
		if n := wordsBefore(whole, c.right); n > 0 && n+1 < len(whole.Words) && relativeAfterNoun(whole, n) {
			continue
		}
		// A cut at a comma parts a clause from the next, so it beats a longer cut inside a phrase.
		comma := seamBefore(source, c.left) == ","
		if bestWords < 0 || comma && !bestComma || comma == bestComma && c.words > bestWords {
			best, bestWords, bestComma = c, c.words, comma
		}
	}
	return best, bestWords
}

// hardRest opens the words after a cut at right as a sentence, past any dash that opened an aside.
func hardRest(source string, right int) string {
	return capitalizeOpening(strings.TrimLeft(source[right:], " —–"))
}

// hardSeamRank grades a cut for hardDivision. A cut that parts a subject from
// its verb, or a verb from what follows it, ranks lowest: "it. Is carrying".
// A cut at a mark, or before a conjunction or a preposition, ranks highest.
func hardSeamRank(source string, whole *syntax.Sentence, c forceCut) int {
	first := wordFrom(whole, c.right)
	if first < 1 || first >= len(whole.Words) {
		return 0
	}
	next, last := whole.Words[first].Tag, whole.Words[first-1].Tag
	if strings.HasPrefix(next, "VB") || next == "MD" || next == "RP" || next == "POS" || strings.HasPrefix(last, "VB") || last == "MD" {
		return 0
	}
	if strings.ContainsAny(source[c.left:c.right], ",;:—–") || strings.HasSuffix(strings.TrimRight(source[:c.left], " "), ",") ||
		next == "CC" || next == "IN" || next == "WDT" || next == "WRB" {
		return 2
	}
	return 1
}

// carrierOrNone is carrierDivision, except for a sentence that opens with an
// infinitive of purpose ("To pull X, use Y"). That sentence reorders behind "Do
// this", and a carrier would restate the purpose clause's verb in place of the
// main one, so it stands down.
func carrierOrNone(source string, whole *syntax.Sentence, c forceCut) (string, string, int) {
	if len(whole.Words) > 1 && whole.Words[0].Lower() == "to" && strings.HasPrefix(whole.Words[1].Tag, "VB") {
		return source[:c.left], "", 0
	}
	return carrierDivision(source, whole, c)
}

// alsoVerb writes the main verb group with "also" before its last verb, so
// "must ensure" restates as "must also ensure".
func alsoVerb(source string, s *syntax.Sentence, verb syntax.Phrase) string {
	text := source[s.Words[verb.First].Start:s.Words[verb.Last].End]
	at := s.Words[verb.Last].Start - s.Words[verb.First].Start
	return text[:at] + "also " + text[at:]
}

// splitsObject reports a cut between a finite verb and the noun phrase right
// after it, which is the verb's object: "a sentence names | an item".
func splitsObject(s *syntax.Sentence, c forceCut) bool {
	next := wordFrom(s, c.right)
	// A mark between the verb and the words after it ends the verb's phrase.
	if next < 1 {
		return false
	}
	// The tagger reads "names" or "reads" as a plural noun. It is the verb when the words before it hold no other.
	prev := s.Words[next-1].Tag
	if !finiteVerbTag(prev) && !(prev == "NNS" && !finiteBetween(s, 0, next-1)) {
		return false
	}
	switch tag := s.Words[next].Tag; {
	case tag == "DT" || tag == "PRP$" || tag == "CD" || strings.HasPrefix(tag, "JJ") || strings.HasPrefix(tag, "NN"):
		return true
	}
	return false
}

// closeHead ends the first part of a division as a sentence. A part that
// already ends on a stop keeps it.
func closeHead(head string) string {
	head = strings.TrimRight(head, " ,;:—–-")
	if strings.HasSuffix(head, ".") || strings.HasSuffix(head, "!") || strings.HasSuffix(head, "?") {
		return head
	}
	return head + "."
}

// divides reports whether Check reads left as a sentence of its own under
// limit. It reads only the start of right, because the rest of it is unchanged.
func divides(left, right string, limit int) bool {
	sentences := Sentences(checkMask(left + " " + opening(right)))
	if len(sentences) < 2 {
		return false
	}
	words := WordCount(sentences[0])
	return words <= limit && words == WordCount(checkMask(left))
}

// inBold reports whether byte p of a sentence sits inside bold text. A bold
// run can open in an earlier sentence, so a first marker that closes a word
// means the sentence opens in bold.
func inBold(source string, p int) bool {
	at := strings.Index(source, "**")
	bold := at > 0 && !unicode.IsSpace(rune(source[at-1])) && (at+2 == len(source) || !isWordByte(source[at+2]))
	for at >= 0 && at < p {
		bold = !bold
		next := strings.Index(source[at+2:], "**")
		if next < 0 {
			break
		}
		at += 2 + next
	}
	return bold
}
