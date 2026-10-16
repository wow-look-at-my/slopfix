package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

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
