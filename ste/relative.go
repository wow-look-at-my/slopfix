package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// takesObject reports a tag that opens the object of the verb before it.
func takesObject(tag string) bool {
	return tag == "DT" || tag == "PRP$" || tag == "PRP" || tag == "CD" || strings.HasPrefix(tag, "NN")
}

// finiteOutsideReduced reports a finite verb from word from up to end that no
// reduced relative clause holds.
func finiteOutsideReduced(s *syntax.Sentence, from, end int) bool {
	for i := max(from, 1); i < end && i < len(s.Words); i++ {
		if finiteAt(s, i) && !inReducedRelative(s, i) {
			return true
		}
	}
	return false
}

// fragmentSubject names the noun phrase a sentence opens on again, with "the"
// and the form of "be" that agrees with it: "The trailing run is".
func fragmentSubject(source string, s *syntax.Sentence) (string, bool) {
	ph, ok := s.PhraseAt(0)
	if !ok || ph.Kind != syntax.NounPhrase || ph.First != 0 || s.Words[0].Tag != "DT" || ph.Last < 1 {
		return "", false
	}
	verb := "is"
	if s.Plural(*ph) {
		verb = "are"
	}
	return "The " + source[s.Words[1].Start:s.Words[ph.Last].End] + " " + verb, true
}

// finiteOutsideRelative reports a finite verb from word from on that no
// relative clause holds. A relative clause holds one finite verb group, and a
// comma ends it. A later finite verb is the main verb: "the file that the
// hook reads is".
func finiteOutsideRelative(s *syntax.Sentence, from int) bool {
	inside, held := false, false
	for i := from; i < len(s.Words); i++ {
		w := s.Words[i]
		switch {
		case w.Text == ",":
			inside, held = false, false
		case w.Tag == "WDT" || w.Tag == "WP" || w.Lower() == "that" && i > 0 && strings.HasPrefix(s.Words[i-1].Tag, "NN"):
			inside, held = true, false
		case !finiteAt(s, i):
		case !inside:
			return true
		case held && !strings.HasPrefix(s.Words[i-1].Tag, "VB") && s.Words[i-1].Tag != "RB":
			return true
		default:
			held = true
		}
	}
	return false
}

// inReducedRelative reports a finite verb at i whose subject follows a noun:
// "a fork the owner reserved".
func inReducedRelative(s *syntax.Sentence, i int) bool {
	k := i - 1
	if ph, ok := s.PhraseAt(k); ok && ph.Kind == syntax.NounPhrase {
		k = ph.First
	} else if s.Words[k].Tag != "PRP" {
		return false
	}
	return k > 0 && strings.HasPrefix(s.Words[k-1].Tag, "NN")
}
