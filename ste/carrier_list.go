package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// becauseDivision divides at "because" when no other boundary serves, and opens
// the reason behind "This is because". The reason must name a subject and a
// verb; "because of X" is a prepositional phrase, not a reason.
func becauseDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	for i := 1; i+1 < len(whole.Words); i++ {
		if whole.Words[i].Lower() != "because" || !opensSubject(whole, i+1) || !finiteBefore(whole, i+1, "") {
			continue
		}
		head := strings.TrimRight(source[:whole.Words[i].Start], " ,")
		if head == "" {
			continue
		}
		left := closeHead(head)
		rest := "This is " + source[whole.Words[i].Start:]
		if WordCount(left) < minimumHalf || WordCount(rest) < minimumHalf || !divides(left, rest, limit) {
			continue
		}
		return left + " " + rest, true
	}
	return source, false
}

// seamIsDash reports a seam that is a dash, which closes the clause before it.
func seamIsDash(seam string) bool { return dashMark(seam) }

// headNoun answers the head noun of the first noun phrase that ends before
// byte at. A comma before a phrase that describes it names this noun again.
func headNoun(whole *syntax.Sentence, at int) (syntax.Word, bool) {
	n := wordsBefore(whole, at)
	for i := 0; i < n; i++ {
		ph, ok := whole.PhraseAt(i)
		if !ok || ph.Kind != syntax.NounPhrase || ph.First != i || ph.Last >= n || ph.Head >= len(whole.Words) {
			continue
		}
		if h := whole.Words[ph.Head]; strings.HasPrefix(h.Tag, "NN") {
			return h, true
		}
	}
	return syntax.Word{}, false
}

// listRest writes the rest of a list as a sentence of its own: "It also
// covers D and E". The words before the cut end the list with its own
// conjunction, which listRest writes into the head through c's left offset.
func listRest(source string, s *syntax.Sentence, verb syntax.Phrase, c forceCut, rest, seam string) (string, string, bool) {
	// A rest that opens on a verb or a preposition is no noun-list item: "may write delete it", "as in reports whether".
	if r := syntax.Parse(checkMask(rest), nil); len(r.Words) > 0 && (strings.HasPrefix(r.Words[0].Tag, "VB") || r.Words[0].Tag == "MD" || r.Words[0].Tag == "IN") {
		return "", "", false
	}
	conj, ok := listEnd(rest)
	// A rest that opens on a conjunction is a clause the list never reached: "but remembered grants are".
	if !ok || len(topCommas(source[:c.left], s.Words[verb.Last].End)) == 0 || leadingConjunction.MatchString(strings.ToLower(rest)) || strings.HasPrefix(strings.ToLower(rest), "but ") {
		return "", "", false
	}
	// A negation and its auxiliary go with the verb, or "Do not run" comes back as "Also run".
	from := verb.First
	for from > 0 && auxiliaries.Contains(s.Words[from-1].Lower()) {
		from--
	}
	verbText := source[s.Words[from].Start:s.Words[verb.Last].End]
	if colon := strings.LastIndex(source[:c.left], ":"); colon > s.Words[verb.Last].End || seam == ":" {
		return "This also covers " + rest, conj, true
	}
	if verb.Imperative {
		return "Also " + lowerFirst(verbText) + " " + rest, conj, true
	}
	subject := subjectFor(source, checkMask(source), c, s.Words[verb.Head].Tag)
	if subject == "" {
		return "", "", false
	}
	return capitalizeOpening(subject) + " also " + verbText + " " + rest, conj, true
}

// listEnd reports a rest that closes a list, "D, E and F.", and answers its conjunction.
func listEnd(rest string) (string, bool) {
	r := syntax.Parse(rest, nil)
	conj := ""
	for i, w := range r.Words {
		switch w.Tag {
		case "VBZ", "VBD", "MD":
			// A verb of a relative clause describes an item: "a subordinator that opens a clause".
			if !inRelative(r, i) {
				return "", false
			}
		case "CC":
			if l := w.Lower(); l == "and" || l == "or" {
				conj = l
			}
		}
	}
	return conj, conj != ""
}

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
