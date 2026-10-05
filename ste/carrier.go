package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// carrier.go divides a long sentence where the words after the cut are no
// clause: a trailing adverbial, a phrase that describes a noun, or the rest
// of a list.

var (
	// carrierAdverbial words open an adverbial that a carrier can take.
	carrierAdverbial = set.Of(wordsOf("carrier-adverbial")...)
	// carrierPlace words open a phrase of place, which describes the noun before it.
	carrierPlace = set.Of(wordsOf("carrier-place")...)
	// stateVerbs state a fact rather than an action.
	stateVerbs = set.Of(wordsOf("state-verb")...)
	// negations turn a verb group around.
	negations = set.Of("not", "never", "n't", "no")
	// auxiliaries come before the verb they carry.
	auxiliaries = set.Of("do", "does", "did", "not", "never", "n't", "must", "can", "will", "may", "should", "cannot")
)

// opensWithCarrier is what a division behind a carrier adds to a cut's score.
const opensWithCarrier = -6

// carrierDivision writes the words after cut c as a sentence of their own
// behind a carrier. It answers the head the division keeps, the rest, and
// the score the rest adds. The rest is "" when no carrier fits.
func carrierDivision(source string, whole *syntax.Sentence, c forceCut) (string, string, int) {
	head := source[:c.left]
	rest := strings.TrimLeft(source[c.right:], " ")
	seam := seamBefore(source, c.left)
	if rest == "" || seam != "" && seam != "," && seam != ":" {
		return head, "", 0
	}
	first := wordFrom(whole, len(source)-len(rest))
	if first < 1 {
		return head, "", 0
	}
	word := whole.Words[first]
	lower := word.Lower()
	prev, ok := lastWordBefore(whole, c.left)
	if !ok {
		return head, "", 0
	}
	noun := strings.HasPrefix(prev.Tag, "NN")
	main, hasMain := mainVerb(whole, c.left)
	// A phrase that describes the noun before it ends the sentence: no comma, conjunction or clause follows it.
	describes := seam == "" && noun && plainPhrase(whole, first)
	switch {
	case !hasMain && !finiteBetween(whole, 0, len(whole.Words)):
		// A sentence with no verb at all is a noun phrase, and only a phrase that describes a noun in it can move.
		if describes && (carrierPlace.Contains(lower) || word.Tag == "VBN" || word.Tag == "VBG") {
			return head, restate(source, prev, rest), opensWithCarrier
		}
		return head, "", 0
	case !hasMain || splitsCoordination(whole, main, first):
		return head, "", 0
	case lower == "so" && (seam == "," || seam == "") && (main.Imperative || instructs(whole)):
		// A so after an instruction states its purpose, and the purpose goes behind "Do this".
		return head, "Do this " + rest, opensWithCarrier
	case describes && carrierPlace.Contains(lower) && !strings.HasPrefix(strings.ToLower(rest), "in order"):
		return head, restate(source, prev, rest), opensWithCarrier
	case describes && (word.Tag == "VBN" || word.Tag == "VBG") && first+1 < len(whole.Words) && !strings.HasPrefix(whole.Words[first+1].Tag, "NN"):
		return head, restate(source, prev, rest), opensWithCarrier
	case seam == "" && noun && (lower == "that" || lower == "which" || lower == "who") && verbAt(whole, first+1) && plainPhrase(whole, first+2):
		return head, restateBare(source, prev, strings.TrimLeft(rest[len(word.Text):], " ")), opensWithCarrier
	case seam != ":" && carrierAdverbial.Contains(lower) && !StandsAlone(rest):
		return head, carrierFor(whole, main) + " " + rest, opensWithCarrier
	case seam == "," && word.Tag == whole.Words[main.Head].Tag && finiteVerbTag(word.Tag) && listsVerbs(source[:c.left]):
		// The rest of a list of verb groups keeps its verbs behind the subject: "It also collapses X, and hides Y".
		subject := mainSubject(source, whole, main, word.Tag)
		if subject == "" {
			return head, "", 0
		}
		return listHead(head, "and", false, whole, main), capitalizeOpening(subject) + " also " + rest, opensWithCarrier
	case seam == "," || seam == ":":
		if carried, conj, ok := listRest(source, whole, main, c, rest, seam); ok {
			return listHead(head, conj, oxford(rest), whole, main), carried, opensWithCarrier
		}
	}
	return head, "", 0
}

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

// plainPhrase reports words from i to the end of the sentence that hold no
// comma, no conjunction, no subordinator and no finite verb outside a
// parenthesis. Only such a phrase moves behind a restated noun whole.
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
		if subject.Last-subject.First < restateLimit && !subject.Coordinated {
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

// mainVerb answers the verb group of the main clause that holds the words
// before byte at.
func mainVerb(s *syntax.Sentence, at int) (syntax.Phrase, bool) {
	for _, c := range s.Clauses {
		if c.Depth == 0 && c.Verb != nil && s.Words[c.Verb.First].Start < at {
			return *c.Verb, true
		}
	}
	return syntax.Phrase{}, false
}

// verbAt reports a verb at word i, finite or bare.
func verbAt(s *syntax.Sentence, i int) bool {
	return i < len(s.Words) && (strings.HasPrefix(s.Words[i].Tag, "VB") || s.Words[i].Tag == "MD")
}

// negated reports a verb group that holds a negation, or one right after it.
func negated(s *syntax.Sentence, verb syntax.Phrase) bool {
	for i := max(verb.First-1, 0); i <= min(verb.Last+1, len(s.Words)-1); i++ {
		if negations.Contains(s.Words[i].Lower()) {
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

// nounText answers the source text of a word, a whole code span where the
// word is the filler the mask wrote over one.
func nounText(source string, w syntax.Word) string {
	return source[outsideSpans(source, w.Start, false):outsideSpans(source, w.End, true)]
}

// restate writes "That <noun> is <rest>", or "Those <nouns> are <rest>".
func restate(source string, noun syntax.Word, rest string) string {
	det, be := "That", "is"
	if noun.Tag == "NNS" || noun.Tag == "NNPS" {
		det, be = "Those", "are"
	}
	return det + " " + nounText(source, noun) + " " + be + " " + rest
}

// restateBare writes "That <noun> <rest>", where rest opens with the verb of a
// relative clause that already agrees with the noun.
func restateBare(source string, noun syntax.Word, rest string) string {
	det := "That"
	if noun.Tag == "NNS" || noun.Tag == "NNPS" {
		det = "Those"
	}
	return det + " " + nounText(source, noun) + " " + rest
}

// listRest writes the rest of a list as a sentence of its own: "It also
// covers D and E". The words before the cut end the list with its own
// conjunction, which listRest writes into the head through c's left offset.
func listRest(source string, s *syntax.Sentence, verb syntax.Phrase, c forceCut, rest, seam string) (string, string, bool) {
	conj, ok := listEnd(rest)
	if !ok || len(topCommas(source[:c.left], s.Words[verb.Last].End)) == 0 {
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
	for _, w := range r.Words {
		switch w.Tag {
		case "VBZ", "VBD", "MD":
			return "", false
		case "CC":
			if l := w.Lower(); l == "and" || l == "or" {
				conj = l
			}
		}
	}
	return conj, conj != ""
}

// reorderDependent moves an opening subordinate clause behind its main clause,
// into a sentence of its own: "If X, Y." becomes "Y. This happens if X." A main
// clause that points back into the subordinate clause, as "those files" does,
// keeps the order: "If X, Y." becomes "Suppose X. Then Y."
func reorderDependent(source string, whole *syntax.Sentence) (string, bool) {
	if !opensDependent(whole) {
		return source, false
	}
	comma := firstComma(whole)
	if comma < 1 || comma+1 >= len(whole.Words) {
		return source, false
	}
	sub := strings.TrimSpace(source[:whole.Words[comma].Start])
	main := strings.TrimSpace(source[whole.Words[comma].End:])
	if !StandsAlone(main) {
		return source, false
	}
	stop := "."
	if n := len(main); n > 0 && strings.ContainsAny(main[n-1:], ".!?") {
		stop, main = main[n-1:], main[:n-1]
	}
	ms := syntax.Parse(checkMask(main), nil)
	verb, ok := mainVerb(ms, len(main))
	if !ok {
		return source, false
	}
	link := strings.ToLower(firstToken.FindString(sub))
	if (link == "if" || link == "when" || link == "whenever") && pointsBack(ms) {
		return "Suppose " + strings.TrimSpace(sub[len(link):]) + ". Then " + lowerFirst(main) + stop, true
	}
	return capitalizeOpening(main) + stop + " " + carrierFor(ms, verb) + " " + lowerFirst(sub) + ".", true
}

// backReferences are the words that point back to a noun said before them.
var backReferences = set.Of("those", "these", "this", "that", "it", "its", "they", "them", "their", "such")

// pointsBack reports a clause whose first words point back to a noun said before them.
func pointsBack(s *syntax.Sentence) bool {
	for _, w := range s.Words[:min(3, len(s.Words))] {
		if backReferences.Contains(w.Lower()) {
			return true
		}
	}
	return false
}

// subjectDivision writes a long subject as a sentence of its own: "A reader
// arriving at X still deserves Y." becomes "Consider a reader arriving at X.
// That reader still deserves Y." The noun the subject names carries the rest.
func subjectDivision(source string, whole *syntax.Sentence) (string, bool) {
	for _, c := range whole.Clauses {
		if c.Depth != 0 || c.Subject == nil || c.Verb == nil {
			continue
		}
		subj := *c.Subject
		head := whole.Words[subj.Head]
		if subj.First != 0 || subj.Last-subj.First < restateLimit || !strings.HasPrefix(head.Tag, "NN") || head.Tag == "NNP" {
			return source, false
		}
		end := whole.Words[c.Verb.First].Start
		// An adverb right before the verb goes with the verb: "still deserves".
		for i := c.Verb.First - 1; i > subj.Last && whole.Words[i].Tag == "RB"; i-- {
			end = whole.Words[i].Start
		}
		subject := strings.TrimRight(source[:end], " ,")
		if WordCount(checkMask(subject))+1 > SentenceWordCap {
			return source, false
		}
		det := "That"
		if head.Tag == "NNS" || head.Tag == "NNPS" {
			det = "Those"
		}
		return "Consider " + lowerFirst(subject) + ". " + det + " " + nounText(source, head) + " " + source[end:], true
	}
	return source, false
}

// lowerFirst writes the first letter in lower case, unless the word is a name in capitals.
func lowerFirst(s string) string {
	if len(s) > 1 && unicode.IsUpper(rune(s[1])) {
		return s
	}
	return lowerOpening(s)
}
