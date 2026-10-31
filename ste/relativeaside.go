// relativeaside.go restates a relative clause no cut can leave behind its noun.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// relativeWords open a clause that describes a noun before them, so no carrier goes in front of one.
var relativeWords = set.Of("which", "who")

// relativeAsideDivision lifts a relative clause out from between a fronted
// phrase and the main clause. The clause goes behind the main clause as a
// sentence with its noun named again. "On a host, which is where X runs, it
// rescans Y" becomes "On a host, it rescans Y. That host is where X runs". No
// cut reads this shape: the words before the clause hold no verb, and the main
// clause after it needs the phrase.
func relativeAsideDivision(source string, s *syntax.Sentence) (string, bool) {
	if len(s.Words) == 0 || s.Words[0].Tag != "IN" {
		return source, false
	}
	for i := 2; i+1 < len(s.Words); i++ {
		if finiteAt(s, i) {
			return source, false
		}
		if !relativeWords.Contains(s.Words[i].Lower()) || s.Words[i-1].Text != "," {
			continue
		}
		closing := asideClose(s, i+1)
		if closing < 0 {
			return source, false
		}
		kept := closeHead(source[:s.Words[i-1].End] + " " + source[s.Words[closing+1].Start:])
		lifted, ok := relativeRest(source, s, i, s.Words[closing].Start)
		if !ok || !StandsAlone(kept) {
			return source, false
		}
		return kept + " " + closeHead(lifted), true
	}
	return source, false
}

// asideClose answers the comma from word i on that ends a clause, with a main
// clause opening after it. A comma inside a list opens no subject and verb,
// so the clause runs on past it.
func asideClose(s *syntax.Sentence, i int) int {
	for j := i; j+1 < len(s.Words); j++ {
		if s.Words[j].Text == "," && finiteBetween(s, i, j) && opensSubject(s, j+1) && finiteBefore(s, j+1, ",") {
			return j
		}
	}
	return -1
}

// relativeRest writes the relative clause at word which, up to byte end, as a
// sentence that names its noun again. A clause with a subject of its own stands
// behind "one": "That host is one which the owner reserved".
func relativeRest(source string, s *syntax.Sentence, which, end int) (string, bool) {
	noun, ok := antecedent(s, which)
	if !ok {
		return "", false
	}
	if verbAt(s, which+1) {
		return restateBare(source, noun, source[s.Words[which+1].Start:end]), true
	}
	pronoun := "one"
	if noun.Tag == "NNS" || noun.Tag == "NNPS" {
		pronoun = "ones"
	}
	return restate(source, noun, pronoun+" "+source[s.Words[which].Start:end]), true
}

// antecedent answers the head of the earliest noun phrase before the relative
// word at i that agrees with the verb after that word. The noun right before
// the word can sit in a phrase that describes the real one: "a host with no
// inotify, which is where".
func antecedent(s *syntax.Sentence, i int) (syntax.Word, bool) {
	verb := syntax.Phrase{First: i + 1, Last: i + 1}
	for k := 0; k < i; k++ {
		ph, ok := s.PhraseAt(k)
		if !ok || ph.Kind != syntax.NounPhrase || ph.First != k || ph.Head >= len(s.Words) {
			continue
		}
		if h := s.Words[ph.Head]; strings.HasPrefix(h.Tag, "NN") && agrees(s, *ph, verb) {
			return h, true
		}
	}
	return syntax.Word{}, false
}
