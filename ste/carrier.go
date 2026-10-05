package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// carrier.go divides a long sentence where the words. This happens after
// the cut are no clause: a trailing adverbial, a phrase that describes a
// noun, or the rest of a list.

var (
	// carrierAdverbial words open an adverbial that a carrier can take.
	carrierAdverbial = set.Of(wordsOf("carrier-adverbial")...)
	// focusing adverbs bind to the phrase after them: "lands only on the lines".
	focusing = set.Of("only", "just", "even", "also", "still", "exactly", "directly", "right", "mostly", "mainly", "solely")
	// carrierBound words move behind a carrier only before "every" or "each".
	carrierBound = set.Of(wordsOf("carrier-bound")...)
	// carrierPlace words open a phrase of place, which describes the noun before it.
	carrierPlace = set.Of(wordsOf("carrier-place")...)
	// stateVerbs state a fact rather than an action.
	stateVerbs = set.Of(wordsOf("state-verb")...)
	// nominalIng words end a compound noun after a singular noun: "the ste finding".
	nominalIng = set.Of(wordsOf("nominal-ing")...)
	// partitives name a part of a group and take a plural verb: "the rest are".
	partitives = set.Of(wordsOf("partitive")...)
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
	// A head that ends inside a clause still waiting for its verb is no sentence.
	if headOpen(whole, first) {
		return head, "", 0
	}
	noun := strings.HasPrefix(prev.Tag, "NN")
	// An -ing noun after a singular noun is one compound noun with it, and no division lands inside it.
	if prev.Tag == "NN" && nominalIng.Contains(lower) {
		return head, "", 0
	}
	// "A trailing run of turns that each made a call, spaced close together": a participle after a fragment describes the fragment's head.
	if seam == "," && (word.Tag == "VBN" || word.Tag == "VBD") && first+1 < len(whole.Words) && !takesObject(whole, first+1) &&
		!finiteOutsideRelative(syntax.Parse(checkMask(head), nil), 0) {
		if subject, ok := fragmentSubject(source, whole); ok {
			return fragmentHead(head), subject + " " + rest, opensWithCarrier
		}
	}
	main, hasMain := mainVerb(whole, c.left)
	// A phrase after a noun that a verb then follows is the subject's own: "the case above passes".
	predicateFollows := finiteBefore(whole, first+1, ",")
	placeless := predicateFollows || !opensObject(whole, first+1)
	// A phrase that describes the noun before it ends the sentence: no comma, conjunction or clause follows it.
	describes := seam == "" && noun && plainPhrase(whole, first)
	switch {
	case !hasMain && !finiteBetween(whole, 0, len(whole.Words)):
		// A sentence with no verb at all is a noun phrase, and only a phrase that describes a noun in it can move.
		if describes && (carrierPlace.Contains(lower) && !placeless || !predicateFollows && (word.Tag == "VBN" || word.Tag == "VBG")) {
			return head, restate(source, prev, rest), opensWithCarrier
		}
		return head, "", 0
	case !hasMain || splitsCoordination(whole, main, first):
		return head, "", 0
	case lower == "so" && (seam == "," || seam == "") && (main.Imperative || instructs(whole)):
		// A so after an instruction states its purpose, and the purpose goes behind "Do this".
		return head, "Do this " + rest, opensWithCarrier
	case describes && !placeless && carrierPlace.Contains(lower) && !strings.HasPrefix(strings.ToLower(rest), "in order"):
		return head, restate(source, prev, rest), opensWithCarrier
	case describes && !predicateFollows && (word.Tag == "VBN" || word.Tag == "VBG") && first+1 < len(whole.Words) && !strings.HasPrefix(whole.Words[first+1].Tag, "NN"):
		return head, restate(source, prev, rest), opensWithCarrier
	case seam == "" && noun && (lower == "that" || lower == "which" || lower == "who") && verbAt(whole, first+1) && plainPhrase(whole, first+2):
		return head, restateBare(source, prev, strings.TrimLeft(rest[len(word.Text):], " ")), opensWithCarrier
	case seam != ":" && carrierAdverbial.Contains(lower) && !StandsAlone(rest) && adverbialMoves(whole, first, prev, carrierFor(whole, main) == "This holds") &&
		(!inRelative(whole, first-1) || everyNext(whole, first)) && !coordinatedVerb(whole, first):
		// A reason reads behind "This is", whatever the verb: "This is because X".
		if lower == "because" {
			return head, "This is " + rest, opensWithCarrier
		}
		return head, carrierFor(whole, main) + " " + rest, opensWithCarrier
	case seam == "," && word.Tag == whole.Words[main.Head].Tag && finiteVerbTag(word.Tag) && unicode.IsLower(rune(word.Text[0])) &&
		listsVerbs(source[:c.left]) && !conjunctionBetween(whole, main.Last+1, first) && !subordinatorBetween(whole, main.Last+1, first+1):
		// The rest of a list of verb groups keeps its verbs behind the subject: "It also collapses X, and hides Y".
		subject := mainSubject(source, whole, main, word.Tag)
		if subject == "" {
			return head, "", 0
		}
		return listHead(head, "and", false, whole, main), capitalizeOpening(subject) + " also " + rest, opensWithCarrier
	case seam == "," || seam == ":":
		// A list that opens after "so" or a subordinator belongs to that clause: "is a thin wrapper, so a hook, a CI job and an editor integration all get".
		if carried, conj, ok := listRest(source, whole, main, c, rest, seam); ok && !subordinatorBetween(whole, main.Last+1, first+1) {
			return listHead(head, conj, oxford(rest), whole, main), carried, opensWithCarrier
		}
		if seam != ":" || lowerIdentifier(firstToken.FindString(rest)) {
			break
		}
		// After a colon, words with a verb of their own are a sentence the parser missed, as with a gerund subject: "keying on it let".
		if (word.Tag == "VBG" || word.Tag == "PRP") && finiteBefore(whole, first, ",") {
			return head, capitalizeOpening(rest), opensWithCarrier
		}
		// Noun phrases after a colon name what the head speaks of: "the words a repair drops, the phrasings it swaps".
		opensNoun := word.Tag == "DT" || word.Tag == "PRP$" || word.Tag == "JJ" || strings.HasPrefix(word.Tag, "NN")
		// One noun phrase after a colon, whose only verbs sit in its relative clauses, defines the head: "a stated count: a number that is true today".
		if opensNoun && !strings.Contains(rest, ",") && finiteBetween(whole, first, len(whole.Words)) && !finiteOutsideRelative(whole, first) {
			return head, "This is " + rest, opensWithCarrier
		}
		// A clause the tagger misread stands as a sentence of its own.
		if opensNoun && hiddenVerb(whole, first) {
			return head, capitalizeOpening(rest), opensWithCarrier
		}
		if opensNoun && !finiteOutsideReduced(whole, first, len(whole.Words)) {
			return head, "This covers " + rest, opensWithCarrier
		}
	}
	return head, "", 0
}

// clauseOpeners open a clause whose verb a head must hold before it can close.
var clauseOpeners = set.Of("whether", "because", "if", "when", "while", "since", "unless", "although", "though", "whereas", "so", "where")

// headOpen reports words before word end that end inside an unfinished clause.
// This covers a subordinator with no finite verb after it, as in "reports
// whether the words", or a noun followed by a new subject with no verb. This
// happens after it, as in "answers the row the earliest node".
func headOpen(s *syntax.Sentence, end int) bool {
	open := -1
	for i := 0; i < end && i < len(s.Words); i++ {
		w := s.Words[i]
		switch {
		case finiteVerbTag(w.Tag) || w.Tag == "MD":
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
	if first+1 >= len(s.Words) {
		return false
	}
	next := s.Words[first+1]
	if !unicode.IsLetter(rune(next.Text[0])) && next.Text[0] != '`' {
		return false
	}
	// A state verb's "This holds" reads with any preposition. An action's "This happens" needs "every" or "each".
	if carrierBound.Contains(s.Words[first].Lower()) && !state {
		return next.Lower() == "every" || next.Lower() == "each"
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

// plainPhrase reports the words from i to the end of the sentence when they
// are plain. Outside a parenthesis, they hold no comma, no conjunction, no
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

// closesPhrase reports a head that ends a sentence with no verb at all on a
// word that completes its phrase. The sentence was a noun phrase as written,
// and each part stays one.
func closesPhrase(head string, whole *syntax.Sentence) bool {
	n := wordsBefore(whole, len(head))
	if n == 0 || finiteBetween(whole, 0, n) {
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
	// A rest that opens on a verb continues a list of verbs, which the noun-list carrier cannot repeat: "may write delete it".
	if r := syntax.Parse(checkMask(rest), nil); len(r.Words) > 0 && (strings.HasPrefix(r.Words[0].Tag, "VB") || r.Words[0].Tag == "MD") {
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
	// The main clause opens after the earliest comma outside a parenthesis that leaves a sentence: "To do X, or to do Y, see Z".
	depth := 0
	for comma, w := range whole.Words {
		switch w.Text {
		case "(":
			depth++
		case ")":
			depth = max(depth-1, 0)
		}
		if w.Text != "," || depth > 0 || comma < 1 || comma+1 >= len(whole.Words) {
			continue
		}
		sub := strings.TrimSpace(source[:w.Start])
		main := strings.TrimSpace(source[w.End:])
		imperative := opensImperative(checkMask(main))
		if !StandsAlone(main) && !imperative {
			continue
		}
		stop := "."
		if n := len(main); n > 0 && strings.ContainsAny(main[n-1:], ".!?") {
			stop, main = main[n-1:], main[:n-1]
		}
		ms := syntax.Parse(checkMask(main), nil)
		carrier := "Do this"
		if verb, ok := mainVerb(ms, len(main)); ok && !(imperative && !verb.Imperative) {
			carrier = carrierFor(ms, verb)
		} else if !imperative {
			continue
		}
		link := strings.ToLower(firstToken.FindString(sub))
		if (link == "if" || link == "when" || link == "whenever") && pointsBack(ms) {
			return "Suppose " + strings.TrimSpace(sub[len(link):]) + ". Then " + lowerFirst(main) + stop, true
		}
		return capitalizeOpening(main) + stop + " " + carrier + " " + lowerFirst(sub) + ".", true
	}
	return source, false
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
func subjectDivision(source string, whole *syntax.Sentence, limit int) (string, bool) {
	// The subject runs from the opening determiner to the first finite verb. The parser often names a later noun as the subject of a long one.
	if len(whole.Words) == 0 || whole.Words[0].Tag != "DT" {
		return source, false
	}
	verb := -1
	for i, w := range whole.Words {
		// A relative word ahead of the verb makes the verb the relative clause's own: "a sentence that will not fit".
		if w.Text == "," || w.Text == ":" || w.Text == ";" || w.Text == "(" || w.Tag == "WDT" || w.Tag == "WP" || w.Lower() == "that" {
			return source, false
		}
		if finiteVerbTag(w.Tag) || w.Tag == "MD" {
			verb = i
			break
		}
		// A noun followed by a new subject opens a relative clause with no relative
		// word: "a file the rule reports nothing in".
		if i > 0 && strings.HasPrefix(whole.Words[i-1].Tag, "NN") && (w.Tag == "DT" || w.Tag == "PRP" || w.Tag == "PRP$") {
			return source, false
		}
	}
	if verb < restateLimit+1 {
		return source, false
	}
	var head syntax.Word
	found := false
	for _, ph := range whole.Phrases {
		if ph.Kind == syntax.NounPhrase && ph.First == 0 {
			head, found = whole.Words[ph.Head], true
		}
	}
	if !found || !strings.HasPrefix(head.Tag, "NN") || head.Tag == "NNP" {
		return source, false
	}
	first := verb
	// An adverb right before the verb goes with the verb: "still deserves".
	for first > 1 && whole.Words[first-1].Tag == "RB" {
		first--
	}
	end := whole.Words[first].Start
	subject := strings.TrimRight(source[:end], " ")
	if WordCount(checkMask(subject))+1 > limit {
		return source, false
	}
	det := "That"
	if head.Tag == "NNS" || head.Tag == "NNPS" {
		det = "Those"
	}
	return "Consider " + lowerFirst(subject) + ". " + det + " " + nounText(source, head) + " " + source[end:], true
}

// lowerFirst writes the first letter in lower case, unless the word is a name in capitals.
func lowerFirst(s string) string {
	if len(s) > 1 && unicode.IsUpper(rune(s[1])) {
		return s
	}
	return lowerOpening(s)
}
