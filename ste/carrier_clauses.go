// carrier_clauses.go reads which words around a cut open or continue a clause.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// clauseOpeners open a clause whose verb a head must hold before it can close.
var clauseOpeners = set.Of("whether", "because", "if", "when", "while", "since", "unless", "although", "though", "whereas", "so", "where")

// opensAdverbialClause reports "as" or a subordinator at word i whose words hold a
// finite verb before the next comma. "as its go.mod approves" is a clause, and
// no restated noun is it.
func opensAdverbialClause(s *syntax.Sentence, i int) bool {
	lower := s.Words[i].Lower()
	return (lower == "as" || syntax.Is(lower, "subordinator")) && finiteBefore(s, i+1, ",")
}

// headOpen reports words before word end that end inside an unfinished clause.
// One is a subordinator with no finite verb after it: "reports whether the
// words". The other is a noun with a new subject and no verb after them:
// "answers the row the earliest node".
func headOpen(s *syntax.Sentence, end int) bool {
	open := -1
	for i := 0; i < end && i < len(s.Words); i++ {
		w := s.Words[i]
		switch {
		case finiteVerbTag(w.Tag) || w.Tag == "MD":
			open = -1
		case open >= 0 && i > open+1 && w.Tag == "NNS" && s.Words[i-1].Tag == "NN" && i+1 < len(s.Words) && s.Words[i+1].Tag == "IN":
			// The tagger reads the verb of the new subject as a plural noun: "a word the actor of a clause stops in front of".
			open = -1
		case clauseOpeners.Contains(w.Lower()) && i > 0:
			open = i
		case i > 0 && strings.HasPrefix(s.Words[i-1].Tag, "NN") && (w.Tag == "DT" || w.Tag == "PRP"):
			open = i
		}
	}
	return open >= 0
}

// everyNext reports "every" or "each" after the preposition at word i: an
// adverbial of frequency, which belongs to the sentence however deep it sits.
func everyNext(s *syntax.Sentence, i int) bool {
	return i+1 < len(s.Words) && (s.Words[i+1].Lower() == "every" || s.Words[i+1].Lower() == "each")
}

// fragmentHead turns a noun phrase that holds no main verb into a sentence.
func fragmentHead(head string) string {
	return "This is " + lowerOpening(head)
}

// takesObject reports an object after a participle. An adjective with no
// noun after it is a complement: "spaced close enough".
func takesObject(s *syntax.Sentence, i int) bool {
	if s.Words[i].Tag == "JJ" && !opensObject(s, i+1) {
		return false
	}
	return opensObject(s, i)
}

// opensObject reports word i opening a noun phrase a preposition can take.
// This covers a determiner, a possessive, a number, an adjective, a name or a
// singular noun.
func opensObject(s *syntax.Sentence, i int) bool {
	if i >= len(s.Words) {
		return false
	}
	switch s.Words[i].Tag {
	case "DT", "PRP$", "CD", "JJ", "NN", "NNP", "NNPS", "PRP":
		return true
	}
	return s.Words[i].Text[0] == '`'
}

// inRelative reports word i inside a clause a relative word opens, whose
// verb the words after i complete: "the filters that can write a file".
func inRelative(s *syntax.Sentence, i int) bool {
	for _, c := range s.Clauses {
		if c.Kind == syntax.Relative && c.First <= i && i <= c.Last {
			return true
		}
	}
	return false
}

// coordinatedVerb reports ", and" or ", or" before a finite verb from word i
// on. That verb shares the main clause's subject, so the words before it are
// no adverbial of their own: "because X was committed, and returns Y".
func coordinatedVerb(s *syntax.Sentence, i int) bool {
	for ; i+2 < len(s.Words); i++ {
		if s.Words[i].Text == "," && (s.Words[i+1].Lower() == "and" || s.Words[i+1].Lower() == "or") && finiteVerbTag(s.Words[i+2].Tag) {
			return true
		}
	}
	return false
}

// subordinatorBetween reports "so" or a subordinator from word from up to end.
func subordinatorBetween(s *syntax.Sentence, from, end int) bool {
	for i := max(from, 0); i < end && i < len(s.Words); i++ {
		if clauseOpeners.Contains(s.Words[i].Lower()) {
			return true
		}
	}
	return false
}

// hiddenVerb reports a verb the tagger read as something else, from word i on.
// It finds a form of "be", or a plural noun right before a participle. An
// example is "an identifier split across lines stops being either".
func hiddenVerb(s *syntax.Sentence, i int) bool {
	for ; i < len(s.Words); i++ {
		w := s.Words[i]
		switch w.Lower() {
		case "be", "being", "been", "is", "are", "was", "were":
			return true
		}
		if w.Tag == "NNS" && i+1 < len(s.Words) && (s.Words[i+1].Tag == "VBG" || s.Words[i+1].Tag == "VBN") {
			return true
		}
	}
	return false
}

// adverbialMoves reports an adverbial at word first that a carrier can take. It
// never takes one right after a verb or a particle, which the adverbial completes:
// "the question was aimed at", "a repair lands only on the lines". A bound
// preposition moves only before "every" or "each". A preposition with no object
// after it ("aimed at: a prose question") belongs to the words before it.
func adverbialMoves(s *syntax.Sentence, first int, prev syntax.Word, state bool) bool {
	// ", where the fork's changes made": after a comma a clause opener starts its own clause, whatever the verb before it.
	opensClause := first > 0 && s.Words[first-1].Text == "," && clauseOpeners.Contains(s.Words[first].Lower()) && finiteBefore(s, first+1, ",")
	if !opensClause && strings.HasPrefix(prev.Tag, "VB") || prev.Tag == "RP" || focusing.Contains(prev.Lower()) || phraseEndParticle.Contains(prev.Lower()) {
		return false
	}
	// "which is immediately before each request": a phrase after "be" and its adverbs completes "be".
	k := first - 1
	for k >= 0 && s.Words[k].Tag == "RB" {
		k--
	}
	if k >= 0 && beForms.Contains(s.Words[k].Lower()) {
		return false
	}
	if first+1 >= len(s.Words) {
		return false
	}
	next := s.Words[first+1]
	// "with no verb after it, as in": a lone pronoun object points back to the noun before it, and a carrier loses that noun.
	if next.Tag == "PRP" && (first+2 >= len(s.Words) || s.Words[first+2].Tag == "," || s.Words[first+2].Tag == ".") {
		return false
	}
	if !unicode.IsLetter(rune(next.Text[0])) && next.Text[0] != '`' {
		return false
	}
	// A state verb's "This holds" reads with any preposition. An action's "This happens" needs "every" or "each", or a comma that already parts the phrase from the verb.
	if carrierBound.Contains(s.Words[first].Lower()) && !state {
		return next.Lower() == "every" || next.Lower() == "each" || first > 0 && s.Words[first-1].Text == ","
	}
	return true
}

// finiteBefore reports a finite verb from word i on, outside a parenthesis, ahead of the first stop mark.
func finiteBefore(s *syntax.Sentence, i int, stop string) bool {
	depth := 0
	for ; i < len(s.Words); i++ {
		switch w := s.Words[i]; {
		case w.Text == "(":
			depth++
		case w.Text == ")":
			depth = max(depth-1, 0)
		case depth > 0:
		case w.Text == stop:
			return false
		case finiteVerbTag(w.Tag) || w.Tag == "MD":
			return true
		}
	}
	return false
}
