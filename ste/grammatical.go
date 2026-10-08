package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// admissible reports whether the left half of a clause division stands as a
// sentence. openerFor already judged the right half.
func admissible(s *syntax.Sentence, source string, d division) bool {
	n := wordsBefore(s, d.leftEnd)
	return !cutsAside(mask(source), d.leftEnd, d.rightStart) && standsAlone(s, 0, n) && segmentStands(s, n)
}

// segmentStands reports whether the words since the last colon or semicolon
// before word end hold a main clause.
func segmentStands(s *syntax.Sentence, end int) bool {
	for end > 0 && strings.Contains(":;,", s.Words[end-1].Text) && s.Words[end-1].Text != "" {
		end--
	}
	from := segmentStart(s, end)
	return from == 0 || standsAlone(s, from, end)
}

// StandsAlone reports whether text holds a main clause, as a sentence must.
// "If the cache is cold" and "the request builder that fits the budget" do not.
func StandsAlone(text string) bool {
	masked := mask(text)
	s := syntax.Parse(masked, opaque(text, masked))
	return standsAlone(s, 0, len(s.Words))
}

// wordsBefore counts the words of s that start before byte at.
func wordsBefore(s *syntax.Sentence, at int) int {
	n := 0
	for n < len(s.Words) && s.Words[n].Start < at {
		n++
	}
	return n
}

// skipAdverbs answers the earliest word from i that is not an adverb.
func skipAdverbs(s *syntax.Sentence, i int) int {
	for i < len(s.Words) && s.Words[i].Tag == "RB" {
		i++
	}
	return i
}

// subjectFollows reports whether clause c names its subject right after its
// link, and the subject opens with a word that can open one.
func subjectFollows(s *syntax.Sentence, c syntax.Clause) bool {
	i := skipAdverbs(s, c.Link+1)
	if c.Subject == nil || c.Verb == nil || c.Subject.First != i || !opensSubject(s, i) {
		return false
	}
	return verbOfSubject(s, *c.Subject, *c.Verb)
}

// verbOfSubject reports whether verb is the verb of subject. Consider a
// pronoun or a verb between them. That pronoun opens another clause, as in
// "a note so it is read", and the parser has paired the verb of that clause
// with the wrong subject.
func verbOfSubject(s *syntax.Sentence, subject, verb syntax.Phrase) bool {
	for j := describedBy(s, subject.Last+1, verb.First); j < verb.First; j++ {
		if tag := s.Words[j].Tag; tag == "PRP" || tag == "WDT" || tag == "WP" || tag == "MD" || strings.HasPrefix(tag, "VB") {
			return false
		}
	}
	return true
}

// describedBy answers the word after a clause that describes the subject and
// sits between from and the verb at to, or from. One kind sits between commas:
// ", which sends stderr to the terminal,". The other has no relative word and
// a finite verb of its own: "the message it writes to comply puts".
func describedBy(s *syntax.Sentence, from, to int) int {
	w := s.Words
	if from+1 < to && w[from].Text == "," && (w[from+1].Tag == "WDT" || w[from+1].Tag == "WP") {
		for j := from + 2; j < to; j++ {
			if w[j].Text == "," {
				return j + 1
			}
		}
		return from
	}
	// "a map that carries no information is": a bare relative clause with its own verb.
	relative := from < to && (w[from].Tag == "WDT" || w[from].Tag == "WP" || w[from].Lower() == "that")
	if from >= to || !relative && w[from].Tag != "PRP" && w[from].Tag != "DT" && w[from].Tag != "PRP$" {
		return from
	}
	for j := from + 1; j < to; j++ {
		if t := w[j].Tag; t == "," || t == ":" || t == "." || t == "CC" || t == "WDT" || t == "WP" {
			return from
		}
	}
	if finiteBetween(s, from+1, to) {
		return to
	}
	return from
}

// reducedRelative reports a clause whose subject follows a noun of the same
// clause, as in "the files you expect to touch".
func reducedRelative(s *syntax.Sentence, c syntax.Clause) bool {
	i := c.Subject.First - 1
	return i >= c.First && i > c.Link && strings.HasPrefix(s.Words[i].Tag, "NN")
}

// laterVerb reports a finite verb in clause c after the verb the parser gave
// it. That first "verb" then modifies a noun, as in "and remembered grants are
// not consulted", and the clause has a subject of its own.
func laterVerb(s *syntax.Sentence, c syntax.Clause) bool {
	for j := c.Verb.Last + 1; j <= c.Last && j < len(s.Words); j++ {
		if tag := s.Words[j].Tag; tag == "VBZ" || tag == "VBP" || tag == "MD" {
			return true
		}
	}
	return false
}

// indexOf answers the place of clause c among the clauses of s.
func indexOf(s *syntax.Sentence, c syntax.Clause) int {
	for k, other := range s.Clauses {
		if other.First == c.First {
			return k
		}
	}
	return -1
}

// agrees reports whether a verb agrees in number with its subject. A clause
// that does not agree, as in "your own investigation are", is half of a
// compound subject that the parser cut apart.
func agrees(s *syntax.Sentence, subject, verb syntax.Phrase) bool {
	for i := verb.First; i <= verb.Last; i++ {
		switch s.Words[i].Tag {
		case "VBZ":
			return !s.Plural(subject)
		case "VBP":
			lower := s.Words[subject.Head].Lower()
			return s.Plural(subject) || lower == "i" || lower == "you" || partitives.Contains(lower)
		case "MD", "VBD", "VB":
			return true
		}
	}
	return true
}

// verbFollows reports whether clause c opens on its verb right after its link.
func verbFollows(s *syntax.Sentence, c syntax.Clause) bool {
	if c.Verb == nil {
		return false
	}
	i := c.Link + 1
	for i < c.Verb.First && s.Words[i].Tag == "RB" {
		i++
	}
	return c.Verb.First == i
}

// asideDash matches a dash that opens or closes an aside.
var asideDash = regexp.MustCompile(`—|–| -- | - `)

// asides answers each span from a dash to the dash after it. A last dash with
// no partner closes nothing, so it is a seam rather than an aside.
func asides(text string) [][]int {
	dashes := asideDash.FindAllStringIndex(text, -1)
	var out [][]int
	for i := 0; i+1 < len(dashes); i += 2 {
		out = append(out, []int{dashes[i][0], dashes[i+1][1]})
	}
	return out
}

// cutsAside reports whether a division leaves part of an aside on each side.
// A division at the closing dash ends the aside with the sentence, which reads.
func cutsAside(masked string, leftEnd, rightStart int) bool {
	for _, a := range asides(masked) {
		if a[0] < rightStart && leftEnd < a[1] && rightStart < a[1] {
			return true
		}
	}
	return false
}

// standsAlone reports whether the words of s from from up to end hold a main
// clause: a subject and a finite verb, or an imperative. Words that open on a
// subordinate clause or on a "to" infinitive need that main clause after a
// comma, because "If the cache is cold" alone is a fragment. It reads the parse
// of the whole sentence, because a fragment parsed alone gets other tags.
func standsAlone(s *syntax.Sentence, from, end int) bool {
	after := from - 1
	if opensDependentAt(s, from) {
		after = firstCommaFrom(s, from)
		if after < 0 || after >= end {
			return false
		}
	}
	// "Do NOT modify the workspace": the tagger reads a sentence-initial "Do" as a name.
	if lead := skipAdverbs(s, from); lead+1 < end && s.Words[lead].Lower() == "do" {
		if v := skipAdverbs(s, lead+1); v < end && s.Words[v].Tag == "VB" {
			return true
		}
	}
	for _, c := range s.Clauses {
		if c.Depth != 0 || c.Verb == nil || c.Verb.Last >= end || c.Verb.First < from {
			continue
		}
		start := -1
		switch {
		case c.Subject != nil && reducedRelative(s, c) && !frontedPhraseOnly(s, from, *c.Subject) && !opensParticiple(s, from):
		case c.Subject != nil:
			start = c.Subject.First
		case c.Verb.Imperative:
			start = c.Verb.First
		case imperativeTag(s, *c.Verb) && skipAdverbs(s, max(c.First, c.Link+1)) == c.Verb.First:
			// "Do NOT modify the workspace": the tagger reads a bare verb at the start as present tense.
			start = c.Verb.First
		case c.Kind == syntax.Opens && c.Verb.Finite && c.Verb.First > c.First && subjectOpensAt(s, c.First, c.Verb.First):
			// The words ahead of a finite verb are its subject, though the tagger missed it: "The comment scan reads".
			start = c.First
		}
		if start > after {
			return true
		}
	}
	return false
}

// frontedPhraseOnly reports words from from up to subject that open on a
// preposition and hold no finite verb, as in "On a web surface every message
// goes".
func frontedPhraseOnly(s *syntax.Sentence, from int, subject syntax.Phrase) bool {
	lead := skipAdverbs(s, from)
	return lead < subject.First && s.Words[lead].Tag == "IN" && !finiteBetween(s, lead, subject.First)
}

// opensParticiple reports words from word from that open on a participle. After
// such a phrase, a subject and its verb are the main clause: "Left in the text
// a rewrite wraps it".
func opensParticiple(s *syntax.Sentence, from int) bool {
	return from < len(s.Words) && (s.Words[from].Tag == "VBN" || s.Words[from].Tag == "VBD")
}

// subjectOpensAt reports a run of words that reads as a subject. An adverb at
// either end of it is none: "No longer exist" is no sentence.
func subjectOpensAt(s *syntax.Sentence, from, verb int) bool {
	if verb-1 < from || adverbLed(s.Words[from]) || adverbLed(s.Words[verb-1]) {
		return false
	}
	return true
}

func adverbLed(w syntax.Word) bool { return w.Tag == "RB" || w.Tag == "JJR" || w.Tag == "JJS" }

// instructs reports a sentence whose main clause is an imperative, after any
// opening subordinate clause: "When a step exists, include a quote". A so in
// an instruction states the purpose of the instruction.
func instructs(s *syntax.Sentence) bool {
	i := 0
	if opensDependent(s) {
		if i = firstComma(s) + 1; i == 0 {
			return false
		}
	}
	for i < len(s.Words) && (s.Words[i].Tag == "RB" || strings.IndexFunc(s.Words[i].Text, unicode.IsLetter) < 0) {
		i++
	}
	// The tagger often reads a bare verb with no subject as present tense.
	return i < len(s.Words) && (s.Words[i].Tag == "VB" || s.Words[i].Tag == "VBP")
}

// opensDependent reports a sentence that opens on a subordinate clause or a "to" infinitive.
func opensDependent(s *syntax.Sentence) bool { return opensDependentAt(s, 0) }

// opensDependentAt reports whether the words from word from open on a subordinate clause or a "to" infinitive.
func opensDependentAt(s *syntax.Sentence, from int) bool {
	first := -1
	for i := from; i < len(s.Words); i++ {
		if w := s.Words[i]; strings.IndexFunc(w.Text, unicode.IsLetter) >= 0 && w.Tag != "RB" {
			first = i
			break
		}
	}
	if first < 0 {
		return false
	}
	if s.Words[first].Lower() == "to" && first+1 < len(s.Words) && strings.HasPrefix(s.Words[first+1].Tag, "VB") {
		return true
	}
	for _, c := range s.Clauses {
		if c.Link == first && c.Kind == syntax.Subordinate {
			return true
		}
	}
	return false
}

func firstComma(s *syntax.Sentence) int { return firstCommaFrom(s, 0) }

// firstCommaFrom answers the index of the earliest comma at or after word from. With none it answers a negative index.
func firstCommaFrom(s *syntax.Sentence, from int) int {
	for i := from; i < len(s.Words); i++ {
		if s.Words[i].Text == "," {
			return i
		}
	}
	return -1
}

// seamBefore answers the mark that ends the words before byte at: a comma, a
// stop or a dash. It answers "" when a word ends them.
func seamBefore(source string, at int) string {
	head := strings.TrimRight(source[:at], " ")
	for _, mark := range []string{",", ";", ":", "—", "–", "--", " -"} {
		if strings.HasSuffix(head, mark) {
			return strings.TrimSpace(mark)
		}
	}
	return ""
}

// danglingTags are the parts of speech that never end a sentence, because each
// opens a phrase the division takes away.
var danglingTags = set.Of[string]("DT", "JJ", "JJR", "JJS", "PRP$", "IN", "CC",
	"TO", "POS", "MD", "WDT", "WP", "WP$", "WRB", "PDT")

// closesWhole reports whether head, the words before a forced cut, closes as a
// sentence. It must hold a main clause and end on a word that completes its
// phrase. At a bare comma it must end in the main clause, because a comma
// inside a subordinate clause joins the items of a list.
func closesWhole(head, seam string, whole *syntax.Sentence) bool {
	w, ok := lastWordBefore(whole, len(head))
	n := wordsBefore(whole, len(head))
	if !ok || danglingTags.Contains(w.Tag) && !predicateAdjective(whole, wordFrom(whole, w.Start)) || !standsAlone(whole, 0, n) || !segmentStands(whole, n) {
		return false
	}
	if seam != "," {
		return true
	}
	for _, c := range whole.Clauses {
		if whole.Words[c.First].Start <= w.Start && w.Start <= whole.Words[c.Last].Start {
			return c.Depth == 0 && c.Verb != nil
		}
	}
	return false
}

// predicateAdjective reports an adjective at i after a form of be, as in "the closer is not prose". It completes its clause, where an adjective before a noun waits for the noun.
func predicateAdjective(s *syntax.Sentence, i int) bool {
	if i < 1 || i >= len(s.Words) || !strings.HasPrefix(s.Words[i].Tag, "JJ") {
		return false
	}
	k := i - 1
	for k > 0 && s.Words[k].Tag == "RB" {
		k--
	}
	return copulas.Contains(s.Words[k].Lower())
}

// copulas are the forms of be that link a subject to an adjective.
var copulas = set.Of("is", "are", "was", "were", "be", "been", "being", "am")

// opensSubject reports whether word i of s can open a subject: a determiner, a
// pronoun, a name, a number or a possessive. A bare adjective or noun more
// often continues the phrase before it.
func opensSubject(s *syntax.Sentence, i int) bool {
	if i < 0 || i >= len(s.Words) {
		return false
	}
	switch s.Words[i].Tag {
	case "DT", "PRP", "PRP$", "NNP", "NNPS", "EX", "CD":
		return true
	case "VBG":
		// "adding a rule is": a gerund is a subject when a finite verb follows it.
		return finiteBefore(s, i+1, ",")
	}
	return i+1 < len(s.Words) && s.Words[i+1].Tag == "POS"
}

// listsVerbs reports a comma after the main verb of head. A verb group after
// the cut then continues a list of verb groups, as in "it reads, writes and
// holds".
func listsVerbs(head string) bool {
	s := syntax.Parse(strings.TrimRight(head, " ,"), nil)
	for _, c := range s.Clauses {
		if c.Depth != 0 || c.Verb == nil {
			continue
		}
		for i := c.Verb.Last + 1; i < len(s.Words); i++ {
			if s.Words[i].Text == "," {
				return true
			}
		}
		return false
	}
	return false
}

// lowerIdentifier reports a word that opens in lower case and reads as a name
// in code. It carries a capital, a digit, an underscore or a dot inside it.
func lowerIdentifier(word string) bool {
	first, width := utf8.DecodeRuneInString(word)
	if !unicode.IsLower(first) {
		return false
	}
	return strings.IndexFunc(strings.TrimRight(word[width:], ".,;:!?)"), func(r rune) bool {
		return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_' || r == '.'
	}) >= 0
}

// opensClause reports a parse whose first clause starts with its own subject
// and carries a finite verb.
func opensClause(s *syntax.Sentence) bool {
	if len(s.Clauses) == 0 {
		return false
	}
	c := s.Clauses[0]
	return c.Subject != nil && c.Verb != nil && c.Subject.First == 0
}

// opensImperative reports a rest that opens on a bare verb, as in "use the
// copy key". An imperative stands as a sentence with a capital and no subject.
// The tagger reads a lower-case bare verb at the start as a noun. The rest is
// read with the subject an imperative leaves out.
func opensImperative(restMasked string) bool {
	s := syntax.Parse("You "+opening(restMasked), nil)
	if len(s.Words) < 2 || len(s.Clauses) == 0 {
		return false
	}
	c := s.Clauses[0]
	// "You be economical": a bare form after the subject is no finite verb, so the parser attaches none.
	if c.Verb == nil && s.Words[1].Lower() == "be" {
		return true
	}
	if c.Subject == nil || c.Subject.First != 0 || c.Verb == nil || c.Verb.First != 1 {
		return false
	}
	tag := s.Words[1].Tag
	return tag == "VB" || tag == "VBP"
}

// startsTheSentence reports whether a clause starts at the sentence's earliest
// word, which an imperative has to: "Write the file".
func startsTheSentence(s *syntax.Sentence, c syntax.Clause) bool {
	for i := 0; i < c.Verb.First; i++ {
		if s.Words[i].Tag != "RB" && s.Words[i].Tag != "``" {
			return false
		}
	}
	return true
}

// opensImperativeMain reports whether the main clause of head is an imperative.
func opensImperativeMain(head string) bool {
	s := syntax.Parse(head, nil)
	for _, c := range s.Clauses {
		if c.Depth == 0 && c.Verb != nil {
			if c.Verb.Imperative {
				return true
			}
			break
		}
	}
	// The tagger reads a bare verb that opens a sentence as a noun: "Capture what is needed".
	if opensImperative(lowerFirst(head)) {
		return true
	}
	// An opening phrase with no verb, then its comma: "For each, give the path".
	if comma := strings.IndexByte(head, ','); comma > 0 && !strings.ContainsAny(head[:comma], "()") {
		opener := syntax.Parse(head[:comma], nil)
		for _, w := range opener.Words {
			if finiteVerbTag(w.Tag) || w.Tag == "MD" || strings.HasPrefix(w.Tag, "VB") {
				return false
			}
		}
		return opensImperative(strings.TrimSpace(head[comma+1:]))
	}
	return false
}

// wordFrom answers the index of the word of s that starts at or after byte at. With no such word it answers a negative index.
func wordFrom(s *syntax.Sentence, at int) int {
	for i, w := range s.Words {
		if w.Start >= at {
			return i
		}
	}
	return -1
}

// lastWordBefore answers the last word of s with a letter that ends at or before byte at, or false.
func lastWordBefore(s *syntax.Sentence, at int) (syntax.Word, bool) {
	for i := len(s.Words) - 1; i >= 0; i-- {
		if w := s.Words[i]; w.End <= at && strings.IndexFunc(w.Text, unicode.IsLetter) >= 0 {
			return w, true
		}
	}
	return syntax.Word{}, false
}
