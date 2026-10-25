// carrier_heads.go builds the head and subject a carrier sentence reuses, and divides at because.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// oxford reports a list that puts a comma before its conjunction.
func oxford(rest string) bool {
	return strings.Contains(rest, ", and ") || strings.Contains(rest, ", or ")
}

// listHead ends the items before a list cut with the list's conjunction:
// "A, B, C," becomes "A, B and C". It joins at the last comma past the verb
// that no parenthesis or code span holds.
func listHead(head, conj string, serial bool, s *syntax.Sentence, verb syntax.Phrase) string {
	head = strings.TrimRight(head, " ,")
	from := s.Words[verb.Last].End
	if colon := strings.LastIndex(head, ":"); colon > from {
		from = colon
	}
	commas := topCommas(head, from)
	if len(commas) == 0 {
		return head
	}
	at := commas[len(commas)-1]
	// The last item already has its conjunction: "hooks, and `ask` rules".
	if leadingConjunction.MatchString(strings.ToLower(strings.TrimLeft(head[at+1:], " "))) {
		return head
	}
	join := " " + conj + " "
	if serial && len(commas) > 1 {
		join = ", " + conj + " "
	}
	return head[:at] + join + strings.TrimLeft(head[at+1:], " ")
}

// topCommas answers each comma of text from byte from on that no parenthesis,
// bracket or code span holds.
func topCommas(text string, from int) []int {
	var out []int
	depth, code := 0, false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '`':
			code = !code
		case '(', '[':
			depth++
		case ')', ']':
			depth = max(depth-1, 0)
		case ',':
			if i >= from && depth == 0 && !code {
				out = append(out, i)
			}
		}
	}
	return out
}

// standsWithMainClause reports text that stands alone and, when it opens on a
// preposition, holds a finite verb outside a relative clause.
func standsWithMainClause(rest string) bool {
	if !StandsAlone(rest) {
		return false
	}
	if startsWithPreposition(rest) {
		return finiteOutsideRelative(syntax.Parse(checkMask(rest), nil), 0)
	}
	return true
}

// relativeTail reports the words from i to the end as the rest of a relative
// clause whose antecedent sits behind the main verb.
func relativeTail(s *syntax.Sentence, i int) bool {
	for _, w := range s.Words[min(i, len(s.Words)):] {
		if w.Tag == "CC" || w.Text == ":" || w.Text == ";" {
			return false
		}
	}
	return true
}

// startsWithPreposition reports text whose first word is a preposition.
func startsWithPreposition(text string) bool {
	s := syntax.Parse(checkMask(text), nil)
	return len(s.Words) > 0 && (s.Words[0].Tag == "IN" || s.Words[0].Tag == "TO")
}

// plainPhrase reports the words from i to the end of the sentence when they// are plain. Outside a parenthesis, they hold no comma, no conjunction, no
// subordinator and no finite verb. Only such a phrase moves behind a restated noun whole.
func plainPhrase(s *syntax.Sentence, i int) bool {
	depth := 0
	for k, w := range s.Words[min(i, len(s.Words)):] {
		switch w.Text {
		case "(":
			depth++
			continue
		case ")":
			depth = max(depth-1, 0)
			continue
		}
		if depth > 0 {
			continue
		}
		switch w.Tag {
		case "CC":
			// A conjunction between nouns keeps the phrase whole. One before a verb opens a clause.
			if joinsVerb(s, i+k+1) {
				return false
			}
			continue
		case ",", ":", "VBZ", "VBP", "VBD", "MD", "WDT", "WP", "WRB":
			return false
		}
		if l := w.Lower(); l == "so" || l == "because" || l == "while" || l == "when" || l == "if" || l == ";" {
			return false
		}
	}
	return true
}

// relativeWithSubject reports a relative word at i whose clause names. A
// subject of its own and runs to the end of the sentence with no comma or
// colon: "that the later run took from a cache and never published".
func relativeWithSubject(s *syntax.Sentence, i int) bool {
	if i+1 >= len(s.Words) || !opensObject(s, i+1) || !finiteBetween(s, i+1, len(s.Words)) {
		return false
	}
	for _, w := range s.Words[i+1:] {
		if w.Text == "," || w.Text == ":" || w.Text == ";" {
			return false
		}
	}
	return true
}

// mainSubject names the subject of the main clause whose verb group is verb
// again. A short subject repeats with "the" for "a". A long one becomes a
// pronoun that agrees with tag. A person in the singular has no pronoun here.
func mainSubject(source string, s *syntax.Sentence, verb syntax.Phrase, tag string) string {
	for _, c := range s.Clauses {
		if c.Verb == nil || c.Verb.First != verb.First || c.Subject == nil {
			continue
		}
		subject := *c.Subject
		if subject.Last >= verb.First || !opensSubject(s, subject.First) {
			return ""
		}
		head := s.Words[subject.Head]
		if head.Tag == "PRP" {
			return lowerOpening(head.Text)
		}
		if restates(s, subject) {
			text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
			if det := s.Words[subject.First].Lower(); det == "a" || det == "an" {
				text = "the" + text[len(det):]
			}
			return text
		}
		switch {
		case tag == "VBP" || tag != "VBZ" && s.Plural(subject):
			return "they"
		case s.Person(subject):
			return ""
		}
		return "it"
	}
	return ""
}

// closesPhrase reports a head that ends a sentence with no verb at all on a
// word that completes its phrase. The sentence was a noun phrase as written,
// and each part stays one.
func closesPhrase(head string, whole *syntax.Sentence) bool {
	n := wordsBefore(whole, len(head))
	// A verb in a relative clause leaves the head a noun phrase: "a run of turns that each made a call".
	if n == 0 || finiteOutsideRelativeIn(whole, 0, n) {
		return false
	}
	// A head that opens on a preposition or a subordinator leads into the clause after it. Only a noun phrase stands as a fragment of its own.
	switch t := whole.Words[0].Tag; {
	case t == "DT" || t == "JJ" || t == "PRP$" || strings.HasPrefix(t, "NN"):
	default:
		return false
	}
	w, ok := lastWordBefore(whole, len(head))
	return ok && strings.HasPrefix(w.Tag, "NN")
}

// conjunctionBetween reports a conjunction or a relative word among words from
// up to end. A list of verb groups holds neither before its last item.
func conjunctionBetween(s *syntax.Sentence, from, end int) bool {
	for i := max(from, 0); i < end && i < len(s.Words); i++ {
		if t := s.Words[i].Tag; t == "CC" && joinsVerb(s, i+1) || t == "WDT" || t == "WP" || t == "IN" && syntax.Is(s.Words[i].Lower(), "subordinator") {
			return true
		}
	}
	return false
}

// finiteVerbTag reports the tag of a finite verb.
func finiteVerbTag(tag string) bool { return tag == "VBZ" || tag == "VBP" || tag == "VBD" }

// joinsVerb reports a verb at word i, after any determiner, adjective or adverb.
func joinsVerb(s *syntax.Sentence, i int) bool {
	for ; i < len(s.Words); i++ {
		switch t := s.Words[i].Tag; {
		case t == "DT" || t == "JJ" || t == "RB" || t == "PRP$":
			continue
		case strings.HasPrefix(t, "VB") || t == "MD" || t == "PRP":
			return true
		}
		return false
	}
	return false
}

// splitsCoordination reports a cut at word first inside the last conjunct of
// a coordination that holds no verb of its own: "and once more after X". The
// conjunct then ends without its adverbial, and reads as no sentence.
func splitsCoordination(s *syntax.Sentence, verb syntax.Phrase, first int) bool {
	for i := first - 1; i > verb.Last; i-- {
		w := s.Words[i]
		if strings.HasPrefix(w.Tag, "VB") || w.Tag == "MD" {
			return false
		}
		if w.Tag == "CC" {
			return true
		}
	}
	return false
}

// carrierFor answers the words that carry an adverbial of the main clause. An
// instruction takes "Do this", a fact takes "This holds", and an event takes
// "This happens". A negated verb takes "This applies", because "Do this" would
// turn the instruction around.
func carrierFor(s *syntax.Sentence, verb syntax.Phrase) string {
	switch {
	case negated(s, verb):
		return "This applies"
	case verb.Imperative:
		return "Do this"
	case stateVerbs.Contains(s.Words[verb.Head].Lower()) && !passive(s, verb):
		return "This holds"
	}
	return "This happens"
}

// passive reports a verb group that holds a past participle, as in "is cleared".
func passive(s *syntax.Sentence, verb syntax.Phrase) bool {
	for i := verb.First; i <= verb.Last; i++ {
		if s.Words[i].Tag == "VBN" {
			return true
		}
	}
	return false
}

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
