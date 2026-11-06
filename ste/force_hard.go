// force_hard.go divides a sentence no clause boundary fits, at the best seam a word boundary offers.

package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// hardDivision divides a sentence that no grammatical division reads. It keeps
// the longest run of leading words under the cap. It closes that run as a
// sentence and opens the rest with a capital. The cap rule then leaves no
// finding standing.
//
// It never ends a half on a word that needs the next one. It never cuts inside
// a code span, a link, a quotation or a parenthesis. candidates answers only
// the gaps outside those spans, so the cut lands between whole words.
func hardDivision(source, masked string, limit int) (string, bool) {
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
		if n := wordsBefore(whole, c.right); n < len(whole.Words) && finiteVerbTag(whole.Words[n].Tag) || insideCompound(source, whole, c) {
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
		// The head must read as a sentence. A rest that reads as none, such as
		// "Through it while this function runs.", takes a carrier below.
		if !standsAsSentence(closeHead(source[:c.left])) {
			continue
		}
		// A rest that opens on a preposition and holds a finite verb only inside a relative clause has no main clause: "With the verb that governs it, laid out".
		if rs := syntax.Parse(opening(hardRest(source, c.right)), nil); len(rs.Words) > 0 && (rs.Words[0].Tag == "IN" || rs.Words[0].Tag == "TO") && !finiteOutsideRelative(rs, 0) {
			continue
		}
		if strandsPhrase(source, c) && !carriesPhrase(source, c) {
			continue
		}
		// The best seam wins, then the longest leading run. The first cut of a tie is deterministic.
		rank := hardSeamRank(source, whole, c)
		if verblessRest(whole, c, source) {
			rank -= verblessPenalty
		}
		if rank > bestRank || rank == bestRank && c.words > bestWords {
			best, bestWords, bestRank = c, c.words, rank
		}
	}
	if bestWords < 0 {
		return lastResortDivision(source, masked, whole, limit)
	}

	left := closeHead(source[:best.left])
	right := hardRest(source, best.right)
	if strandsPhrase(source, best) && carriesPhrase(source, best) {
		right = "This is " + strings.TrimLeft(source[best.right:], " ")
	}
	if left == "" || right == "" {
		return source, false
	}
	return left + " " + right, true
}

// strandsPhrase reports a cut with no mark at its seam whose rest opens on a
// preposition or a subordinator. The words belong to the verb in front of the
// cut, as in "holds "bazel" | for the tool prose calls Bazel". A phrase that
// opens a sentence ends at a comma before the main verb, and no comma there
// leaves the rest a fragment.
func strandsPhrase(source string, c forceCut) bool {
	if strings.ContainsAny(source[c.left:c.right], ",;:\u2014\u2013") || strings.HasSuffix(strings.TrimRight(source[:c.left], " "), ",") {
		return false
	}
	rest := syntax.Parse(opening(hardRest(source, c.right)), nil)
	if len(rest.Words) == 0 || rest.Words[0].Tag != "IN" && rest.Words[0].Tag != "TO" {
		return false
	}
	for _, w := range rest.Words[1:] {
		if w.Text == "," {
			return false
		}
		if finiteVerbTag(w.Tag) || w.Tag == "MD" {
			return true
		}
	}
	return true
}

// carriesPhrase reports a stranded rest that opens on a preposition of place,
// time or purpose. "This is" in front of it writes a sentence: "This is for
// the tool prose calls Bazel". A subordinator, a relative word or a word of
// comparison takes no such carrier.
func carriesPhrase(source string, c forceCut) bool {
	first := strings.ToLower(firstToken.FindString(source[c.right:]))
	switch first {
	case "as", "than", "like", "so":
		return false
	}
	return !syntax.Is(first, "subordinator") && !syntax.Is(first, "relative") && !syntax.Is(first, "coordinator")
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
	mark := strings.ContainsAny(source[c.left:c.right], ",;:—–") || strings.HasSuffix(strings.TrimRight(source[:c.left], " "), ",")
	// A mark before a rest with its own verb ends a clause.
	if mark && holdsFinite(checkMask(hardRest(source, c.right))) {
		return 3
	}
	if mark || next == "CC" || next == "IN" || next == "WDT" || next == "WRB" {
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
	head, rest, score := carrierDivision(source, whole, c)
	// A pronoun carrier names the subject of a verb group. The rest is a sentence only where a verb follows it: "It "" when no noun ends" has none.
	if strings.HasPrefix(rest, "It ") && !standsAsSentence(rest) {
		return head, "", 0
	}
	return head, rest, score
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
