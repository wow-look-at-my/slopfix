package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// connectorOpener reports an opener a conjunction writes, such as "Otherwise,".
func connectorOpener(opener string) bool {
	for _, v := range connectors {
		if v != "" && v == opener {
			return true
		}
	}
	return false
}

// strandsPhrase reports a rest that opens on a preposition with no clause
// after it, as in "Up to the plural noun it counts, as in X". That phrase
// finishes the sentence before the cut and stands as no sentence of its own.
// A fronted phrase that a comma closes before a clause is a sentence.
func strandsPhrase(s *syntax.Sentence, rightStart int, right string) bool {
	n := wordFrom(s, rightStart)
	if n < 0 || n >= len(s.Words) {
		return false
	}
	w := s.Words[n]
	// "up to" and "out of": a particle before a preposition opens a phrase that finishes the words before it.
	next := n + 1
	opens := next < len(s.Words) && (w.Tag == "RB" || w.Tag == "RP" || w.Tag == "IN") && (s.Words[next].Tag == "IN" || s.Words[next].Tag == "TO")
	if !opens || syntax.Is(w.Text, "subordinator") {
		return false
	}
	comma := strings.Index(right, ",")
	return comma < 0 || !StandsAlone(capitalizeOpening(strings.TrimSpace(right[comma+1:])))
}

// contactRelative reports a division with no mark and no link word, between
// a noun and a clause that opens on a determiner: "into the segment no
// pending reader holds". The clause describes the noun, with its "that" left
// out, so a sentence of its own inverts what it says.
func contactRelative(s *syntax.Sentence, source string, d division) bool {
	if strings.TrimSpace(source[d.leftEnd:d.rightStart]) != "" {
		return false
	}
	n := wordFrom(s, d.rightStart)
	if n < 1 || n >= len(s.Words) {
		return false
	}
	return strings.HasPrefix(s.Words[n-1].Tag, "NN") && (s.Words[n].Tag == "DT" || s.Words[n].Tag == "PRP$")
}

// listBefore reports a comma inside the clause before c, which makes ", and"
// the end of a list rather than a join between clauses.
func listBefore(s *syntax.Sentence, c syntax.Clause) bool {
	for i := c.Link - 2; i >= 0 && i >= clauseStart(s, c.Link); i-- {
		// A comma inside a quotation reads as a name, and lists nothing.
		if s.Words[i].Text == "," && s.Words[i].Tag == "," {
			return true
		}
	}
	return false
}
