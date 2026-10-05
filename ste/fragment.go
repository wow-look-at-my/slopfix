package ste

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// fragmentDivision divides a sentence that holds no main clause. Such a
// sentence is a noun phrase as written, so each part stays a noun phrase. The
// cut lands where a phrase ends and the next opens on a determiner. "a cache
// that never empties a cache that never fills" becomes "a cache that never
// empties. A cache that never fills".
func fragmentDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	if len(whole.Words) == 0 || standsAlone(whole, 0, len(whole.Words)) {
		return source, false
	}
	// Only a noun phrase is a fragment of its own. A sentence that opens on "to" or "if" leads into a clause the parser missed.
	switch t := whole.Words[0].Tag; {
	case t == "DT" || t == "JJ" || t == "PRP$" || strings.HasPrefix(t, "NN"):
	default:
		return source, false
	}
	for _, c := range candidates(source, masked, false, limit) {
		if seam := seamBefore(source, c.left); cutsAside(masked, c.left, c.right) || seam != "" && seam != "," {
			continue
		}
		n := wordsBefore(whole, c.left)
		if n < minimumHalf || len(whole.Words)-n < minimumHalf {
			continue
		}
		if !endsBefore(whole, n) || !opensFragment(whole.Words[n]) || opensMainClause(whole, n) {
			continue
		}
		left := closeHead(source[:c.left])
		right := capitalizeOpening(strings.TrimLeft(source[c.right:], " "))
		if !divides(left, right, limit) {
			continue
		}
		return left + " " + right, true
	}
	return source, false
}

// phraseEndParticle words end a phrase, though the tagger can read one as a preposition.
var phraseEndParticle = set.Of(wordsOf("phrase-end-particle")...)

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
