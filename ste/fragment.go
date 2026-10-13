package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// lowerIdentifier reports a word that opens in lower case and reads as a name
// in code. It carries a capital, a digit, an underscore or a dot inside it. A
// single letter other than the article is a variable, such as q.
func lowerIdentifier(word string) bool {
	first, width := utf8.DecodeRuneInString(word)
	if !unicode.IsLower(first) {
		return false
	}
	if strings.TrimRight(word[width:], ".,;:!?)") == "" {
		return first != 'a'
	}
	return strings.IndexFunc(strings.TrimRight(word[width:], ".,;:!?)"), func(r rune) bool {
		return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_' || r == '.'
	}) >= 0
}

// fragmentDivision divides a sentence that holds no main clause. Such a
// sentence is a noun phrase as written, so each part stays a noun phrase. The
// cut lands where a phrase ends and the next opens on a determiner. "a cache
// that never empties a cache that never fills" becomes "a cache that never
// empties. A cache that never fills".
func fragmentDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	if len(whole.Words) == 0 {
		return source, false
	}
	if standsAlone(whole, 0, len(whole.Words)) && WordCount(checkMask(source)) <= limit {
		return source, false
	}
	// Only a noun phrase is a fragment of its own. A sentence that opens on "to" or "if" leads into a clause the parser missed.
	switch t := whole.Words[0].Tag; {
	case t == "DT" || t == "JJ" || t == "PRP$" || strings.HasPrefix(t, "NN"):
	default:
		return source, false
	}
	// A list of noun phrases joined by commas has no clause to divide at, so each comma ends one item and opens the next.
	for _, c := range candidates(source, masked, false, limit) {
		seam := seamBefore(source, c.left)
		if cutsAside(masked, c.left, c.right) {
			continue
		}
		n := wordsBefore(whole, c.left)
		if n < minimumHalf || len(whole.Words)-n < minimumHalf {
			continue
		}
		if seam == "," {
			// A comma opens the next list item only when a determiner or a list
			// adverb follows it. "Progress, when set, is called" is no list.
			if !opensFragment(whole.Words[n]) && !listAdverb.Contains(whole.Words[n].Lower()) {
				continue
			}
		} else if introduces.Contains(seam) {
			// A colon or a dash introduces what follows it, so the cut lands at the
			// mark: the head ends on the word before it.
			if !closesAtMark(whole, n) {
				continue
			}
		} else if !endsBefore(whole, n) || !opensFragment(whole.Words[n]) || opensMainClause(whole, n) {
			continue
		}
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && danglingTags.Contains(w.Tag) && !predicateAdjective(whole, wordFrom(whole, w.Start)) {
			continue
		}
		// A capital renames an identifier written in lower case, so no part opens on one.
		rest := strings.TrimLeft(source[c.right:], " ")
		if lowerIdentifier(firstToken.FindString(rest)) {
			continue
		}
		left := closeHead(source[:c.left])
		right := capitalizeOpening(rest)
		if !divides(left, right, limit) {
			continue
		}
		return left + " " + right, true
	}
	return source, false
}

// phraseEndParticle words end a phrase, though the tagger can read one as a preposition.
var phraseEndParticle = set.Of(wordsOf("phrase-end-particle")...)

// markDivision divides a sentence that holds no main clause at a real clause
// mark: a colon, a dash, or a comma before a conjunction. The sentence is
// already a fragment, so each part stays one. The mark is what closes the part
// before it, and the cut never lands on a word that needs the next one. The
// right part may still run past the cap, because the cap repair divides it
// again on its own.
func markDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	if standsAlone(whole, 0, len(whole.Words)) {
		return source, false
	}
	best, bestWords := "", -1
	for _, c := range candidates(source, masked, false, limit) {
		seam := seamBefore(source, c.left)
		if cutsAside(masked, c.left, c.right) || !sentenceMark(whole, c, seam) {
			continue
		}
		n := wordsBefore(whole, c.left)
		if n < minimumHalf || len(whole.Words)-n < minimumHalf {
			continue
		}
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && danglingTags.Contains(w.Tag) {
			continue
		}
		left := closeHead(source[:c.left])
		// The right part must read as a sentence, or open a list.
		right := carryRest(source[c.right:])
		if left == "" || right == "" || !divides(left, right, limit) {
			continue
		}
		if n > bestWords {
			best, bestWords = left+" "+right, n
		}
	}
	return best, bestWords >= 0
}

// carryRest writes the words after a mark as a sentence of their own, and
// answers "" where they make none. Words that stand alone, or open a list,
// open with a capital. A noun phrase stands behind "This is". Words that open
// on a conjunction add to what the mark ended, so no carrier goes in front of
// them: "This is and the scalars it set" is no sentence. A clause after the
// conjunction stands behind the conjunction's opener instead. A capital renames
// an identifier written in lower case. Words that open on one keep its case,
// and stand only behind the carrier, as a noun phrase: "This is q split over
// the rungs".
func carryRest(rest string) string {
	rest = strings.TrimLeft(rest, " ")
	first := firstToken.FindString(rest)
	if first == "" {
		return ""
	}
	identifier := lowerIdentifier(first)
	if right := capitalizeOpening(rest); !identifier && (opensList(right) || StandsAlone(right)) {
		return right
	}
	word := strings.ToLower(strings.TrimRight(first, ",;:"))
	if opener, joins := connectors[word]; joins {
		return conjoinedClause(word, opener, strings.TrimLeft(rest[len(first):], " "))
	}
	if identifier && presentClause(rest) {
		return ""
	}
	if carried := "This is " + rest; StandsAlone(carried) {
		return carried
	}
	return ""
}

// openDivision writes the rest of a clause division as a sentence, behind the
// division's opener.
func openDivision(source string, d division) string {
	rest := restOf(source, d)
	if d.opener == "" && lowerIdentifier(firstToken.FindString(rest)) {
		return carryRest(rest)
	}
	return joinOpener(d.opener, rest)
}

// absolutePhrase reports a division at a bare comma whose rest holds no verb
// but a regular past form. "each rung's ratio raised to its elasticity" names
// a noun and a participle that describes it. With no auxiliary, the past form
// reads as a participle, and the rest is a phrase of the sentence before it.
func absolutePhrase(source string, d division) bool {
	if d.opener != "" || strings.TrimSpace(source[d.leftEnd:d.rightStart]) != "," {
		return false
	}
	s := syntax.Parse(checkMask(restOf(source, d)), nil)
	past := false
	for _, w := range s.Words {
		switch {
		case w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "MD":
			return false
		case w.Tag == "VBD" && strings.HasSuffix(w.Lower(), "ed"):
			past = true
		case w.Tag == "VBD":
			return false
		}
	}
	return past
}

// conjoinedClause writes the clause after a conjunction as a sentence behind
// the conjunction's opener, and answers "" where the words are no clause. A so
// states a consequence that a bare period loses, so it never divides here.
func conjoinedClause(conjunction, opener, clause string) string {
	if conjunction == "so" || clause == "" || lowerIdentifier(firstToken.FindString(clause)) {
		return ""
	}
	if !StandsAlone(capitalizeOpening(clause)) && !verbAfterRelative(clause) {
		return ""
	}
	return joinOpener(opener, clause)
}

// verbAfterRelative reports a subject whose relative clause ends on its verb,
// right before the main verb in its bare form: "the rules that follow give".
// The parse reads both verbs into the relative clause, and the main clause then
// has none.
func verbAfterRelative(clause string) bool {
	s := syntax.Parse(checkMask(opening(clause)), nil)
	for i := 1; i+2 < len(s.Words); i++ {
		w := s.Words[i]
		if w.Tag != "WDT" && w.Lower() != "that" || !strings.HasPrefix(s.Words[i-1].Tag, "NN") {
			continue
		}
		relative, main := s.Words[i+1], s.Words[i+2]
		if finiteVerbTag(relative.Tag) && !auxiliaryVerbs.Contains(relative.Lower()) && (main.Tag == "VB" || main.Tag == "VBP") {
			return true
		}
	}
	return false
}

// presentClause reports words that hold a verb in the present tense, or a
// modal, outside a relative clause, a subordinate clause and a parenthesis.
// They read as a clause, and no carrier makes a noun phrase of them. A past
// form can be a participle that describes the noun before it, as in "q split
// over the rungs", so it does not count.
func presentClause(text string) bool {
	s := syntax.Parse(checkMask(opening(text)), nil)
	// An -s form after a determiner and an adjective is a plural noun: "the free rungs".
	for i := 2; i < len(s.Words); i++ {
		if (s.Words[i].Tag == "VBZ" || s.Words[i].Tag == "VBP") && s.Words[i-1].Tag == "JJ" && s.Words[i-2].Tag == "DT" {
			s.Words[i].Tag = "NNS"
		}
	}
	inside, depth := false, 0
	for i, w := range s.Words {
		switch {
		case w.Text == "(":
			depth++
		case w.Text == ")":
			depth = max(depth-1, 0)
		case depth > 0:
		case w.Text == ",":
			inside = false
		case w.Tag == "WDT" || w.Tag == "WP" || w.Tag == "WP$" || w.Tag == "WRB" || w.Lower() == "that" || syntax.Is(w.Lower(), "subordinator"):
			inside = true
		case i > 0 && !inside && (w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "MD") && !inReducedRelative(s, i):
			return true
		}
	}
	return false
}

// opensList reports a right part that opens a list item, whose own text is no
// sentence: "- `/dev` (a terminal, a PTY) — read and write."
func opensList(right string) bool {
	trimmed := strings.TrimLeft(right, " ")
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '-', '*', '+', '>':
		return len(trimmed) > 1 && trimmed[1] == ' '
	}
	return false
}

// sentenceMark reports a cut at a mark that closes the part before it: a colon
// or a dash anywhere, or a comma before a conjunction. A bare comma between
// list items is not one, because a list is not sentences.
func sentenceMark(whole *syntax.Sentence, c forceCut, seam string) bool {
	if seam == ":" || seam == "—" || seam == "–" || seam == "--" || seam == "-" {
		return true
	}
	if seam != "," {
		return false
	}
	n := wordsBefore(whole, c.right)
	if n >= len(whole.Words) {
		return false
	}
	word := strings.ToLower(whole.Words[n].Lower())
	if _, ok := connectors[word]; ok {
		return true
	}
	if word == "which" || word == "who" || word == "that" {
		return true
	}
	return subordinatorWords.Contains(word)
}

// subordinatorWords open a dependent clause, so a comma before one joins clauses.
var subordinatorWords = set.Of("because", "although", "though", "since", "while", "whereas", "unless", "if", "when", "where", "after", "before", "once")

// listAdverb words continue a list without a determiner.
var listAdverb = set.Of("then", "next", "finally", "lastly", "also", "plus")

// introduces are the marks that introduce what follows them. A cut lands at one when no clause boundary serves.
var introduces = set.Of(":", "—", "–", "--", "-")

// closesAtMark reports whether word n of s is an introducing mark and the
// word before it closes a whole phrase.
func closesAtMark(s *syntax.Sentence, n int) bool {
	if n < 2 || !introduces.Contains(s.Words[n-1].Text) {
		return false
	}
	return endsBefore(s, n-1)
}

// PhraseHead answers the longest leading run of text, at most maxWords long,
// that ends a noun phrase where the next word opens one. It closes the run
// with a stop. It serves prose that never closes, which no division reads.
func PhraseHead(text string, maxWords int) (string, bool) {
	masked := checkMask(text)
	s := syntax.Parse(masked, nil)
	for n := min(maxWords, len(s.Words)-1); n >= minimumHalf; n-- {
		if !endsBefore(s, n) || !opensFragment(s.Words[n]) || opensMainClause(s, n) {
			continue
		}
		if seamBefore(text, s.Words[n].Start) != "" {
			continue
		}
		head := closeHead(text[:s.Words[n-1].End])
		if WordCount(checkMask(head)) <= maxWords {
			return head, true
		}
	}
	return "", false
}

// endsPhrase reports a word a noun phrase can end on: a noun, an adverb, a
// number or a particle. A verb wants the object a cut would take away.
func endsPhrase(w syntax.Word) bool {
	// A negation turns the words after it around, so it never ends a phrase.
	if negations.Contains(w.Lower()) {
		return false
	}
	tag := w.Tag
	return strings.HasPrefix(tag, "NN") || strings.HasPrefix(tag, "RB") && trueAdverb(w) || tag == "CD" || phraseEndParticle.Contains(w.Lower())
}

// trueAdverb reports a word the tagger read as an adverb that reads as one:
// it ends in "ly" or is a known adverb.
func trueAdverb(w syntax.Word) bool {
	lower := w.Lower()
	return strings.HasSuffix(lower, "ly") || plainAdverbs.Contains(lower)
}

// auxiliaryVerbs want the words after them, so none of them closes a phrase.
var auxiliaryVerbs = set.Of("is", "are", "was", "were", "be", "been", "being", "am",
	"has", "have", "had", "does", "do", "did")

// plainAdverbs end a phrase though they carry no "ly".
var plainAdverbs = set.Of("here", "there", "now", "too", "today", "later", "first", "last",
	"once", "twice", "ever", "yet", "already", "alone", "else", "instead", "anyway", "soon", "again")

// endsBefore reports a cut before word n that leaves a whole noun phrase. A
// verb ends one only when the words after it open a phrase with a relative
// clause of its own. The verb closed the clause before it: "a cache that
// never empties | a cache that never fills". After "a session that asks", the
// words name its object.
func endsBefore(s *syntax.Sentence, n int) bool {
	if endsPhrase(s.Words[n-1]) {
		return true
	}
	if !strings.HasPrefix(s.Words[n-1].Tag, "VB") || auxiliaryVerbs.Contains(s.Words[n-1].Lower()) {
		return false
	}
	// Only a verb inside a relative clause can close the phrase that holds it. A main verb wants its object.
	relative := false
	for _, w := range s.Words[:n-1] {
		switch w.Lower() {
		case "that", "which", "who":
			relative = true
		}
	}
	if !relative {
		return false
	}
	for j := n + 1; j < len(s.Words); j++ {
		switch w := s.Words[j]; {
		case w.Lower() == "that" || w.Lower() == "which" || w.Lower() == "who":
			return j > n+1
		case strings.HasPrefix(w.Tag, "NN") || strings.HasPrefix(w.Tag, "JJ"):
		default:
			return false
		}
	}
	return false
}

// opensMainClause reports words from i on that hold a subject and its finite
// verb before any stop mark. After a noun they finish that noun as a clause
// with no relative word: "the way the Read tool does", "a word the write did
// not touch".
func opensMainClause(s *syntax.Sentence, i int) bool {
	for j := i + 1; j < len(s.Words); j++ {
		w := s.Words[j]
		switch {
		case strings.ContainsAny(w.Text, ",;:.!?()—–"):
			return false
		case w.Tag == "WDT" || w.Tag == "WP" || w.Lower() == "that":
			// A relative word puts the verb after it inside the phrase.
			return false
		case finiteVerbTag(w.Tag) || w.Tag == "MD":
			return true
		}
	}
	return false
}

// opensFragment reports a word that opens a new noun phrase.
func opensFragment(w syntax.Word) bool {
	switch w.Lower() {
	case "a", "an", "the", "every", "each", "any", "some", "no":
		return true
	}
	return w.Tag == "PRP$"
}
