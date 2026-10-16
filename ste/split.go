package ste

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/codespan"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// A division ends the left sentence at leftEnd. The right sentence opens with
// opener, then the source from rightStart.
type division struct {
	leftEnd, rightStart int
	opener              string
	// closer is the object a division writes back at the end of the rest: ", which a list does not show" becomes "A list does not show that."
	closer string
	// comma is the byte offset where the rest takes a comma after an opening participle phrase: "Dropped, it leaves". Zero means none.
	comma int
}

func participleComma(s *syntax.Sentence, c syntax.Clause) int {
	p := c.Link + 1
	if p+2 >= len(s.Words) || s.Words[p].Tag != "VBN" && s.Words[p].Tag != "VBD" {
		return -1
	}
	if c.Subject == nil {
		// "and dropped it leaves": the pronoun after the participle is the subject.
		if s.Words[p+1].Tag == "PRP" && finiteAt(s, p+2) {
			return p + 1
		}
		return -1
	}
	if c.Subject.First > p+1 {
		return c.Subject.First
	}
	return -1
}

// restOf answers the rest of the source a division writes, with its comma.
func restOf(source string, d division) string {
	if d.comma <= d.rightStart {
		return source[d.rightStart:]
	}
	return source[d.rightStart:d.comma] + "," + source[d.comma:]
}

// dashMark reports a dash, which introduces a clause the sentence before it
// stands without.
func dashMark(text string) bool {
	return text == "--" || text == "—" || text == "–" || text == "-"
}

// connectors map a conjunction to the words that open the sentence after it.
// A period says what and and so say, so both drop.
var connectors = map[string]string{
	"and": "",
	"but": "However,",
	"yet": "However,",
	"so":  "",
	"or":  "Otherwise,",
}

// fixSentenceCap divides every over-cap sentence. It tries the clause
// boundaries first and then the word boundaries. A sentence that no division
// reads is cut between words. That leaves no finding standing.
func fixSentenceCap(prose string, d capSpec) string {
	for range len(strings.Fields(prose)) + 1 {
		next, divided := divideNext(prose, d.cap)
		if !divided {
			break
		}
		prose = next
	}
	return forceSentenceCap(prose, d)
}

// divideNext divides the earliest over-cap sentence that a clause boundary can divide.
func divideNext(prose string, limit int) (string, bool) {
	masked := mask(prose)
	off := opaque(prose, masked)
	for _, span := range sentenceSpans(prose) {
		start, end := span[0], span[1]
		if WordCount(masked[start:end]) <= limit {
			continue
		}
		s := syntax.Parse(masked[start:end], shift(off, -start))
		if rewritten, ok := bestDivision(s, prose[start:end]); ok && keepsCodeSpans(prose[start:end], rewritten) {
			return prose[:start] + rewritten + prose[end:], true
		}
		if rewritten, ok := frontedDivision(s, prose[start:end]); ok && keepsCodeSpans(prose[start:end], rewritten) {
			return prose[:start] + rewritten + prose[end:], true
		}
	}
	return prose, false
}

// keepsCodeSpans reports that a division kept each code span whole, in order.
func keepsCodeSpans(source, rewritten string) bool {
	return slices.Equal(codeSpanTexts(source), codeSpanTexts(rewritten))
}

// codeSpanTexts answers the text of each code span in s, in source order.
func codeSpanTexts(s string) []string {
	var out []string
	for _, span := range codespan.Find(s) {
		out = append(out, s[span[0]:span[1]])
	}
	return out
}

// sentenceSpans answers the byte range of each sentence. It reads the source
// rather than a mask, because a masked code span opens a sentence.
func sentenceSpans(prose string) [][2]int {
	var out [][2]int
	at := 0
	for _, sentence := range Sentences(prose) {
		start := strings.Index(prose[at:], sentence)
		if start < 0 {
			break
		}
		start += at
		at = start + len(sentence)
		out = append(out, [2]int{start, at})
	}
	return out
}

// opaque are the spans the parser reads as names: the data Check hides, a
// parenthetical, and a quotation, whose words belong to somebody else.
func opaque(prose, masked string) [][]int {
	off := offLimits(prose, masked)
	for _, span := range cardinal.QuotedSpans(prose) {
		off = append(off, []int{span.Start, span.End})
	}
	for _, loc := range curlyQuote.FindAllStringIndex(prose, -1) {
		off = append(off, loc)
	}
	return off
}

var curlyQuote = regexp.MustCompile(`“[^”]*”`)

// bestDivision picks the division that leaves the longer half shortest.
func bestDivision(s *syntax.Sentence, source string) (string, bool) {
	best, bestScore := "", -1
	for _, d := range divisions(s, source) {
		// A division inside bold text leaves each half with an unclosed marker.
		if strings.Count(source[:d.leftEnd], "**")%2 != 0 {
			continue
		}
		// A bold run that opens on the conjunction the division drops opens the rest instead: "X **and Y is flagged**" becomes "X. **Y is flagged**".
		bold := ""
		if dropped := strings.TrimSpace(source[d.leftEnd:d.rightStart]); strings.Contains(dropped, "**") {
			if strings.Count(dropped, "**") != 1 || !strings.HasPrefix(dropped, "**") {
				continue
			}
			bold = "**"
		}
		if !admissible(s, source, d) {
			continue
		}
		// "with no visibility test": with no mark or conjunction at the seam, a bare verb form after a noun continues the noun phrase.
		if cutsNounPhrase(source, s, d.leftEnd, d.rightStart) || d.opener == "" && strings.TrimSpace(source[d.leftEnd:d.rightStart]) == "" && opensImperative(checkMask(lowerFirst(restOf(source, d)))) {
			continue
		}
		left := strings.TrimRight(source[:d.leftEnd], " ,;:—–-") + "."
		right := bold + withCloser(joinOpener(d.opener, restOf(source, d)), d.closer)
		if WordCount(left) < minimumHalf || WordCount(right) < minimumHalf {
			continue
		}
		// The parse of the whole can hand a clause a verb that belongs elsewhere, so each half is read again on its own.
		if !standsAsSentence(left) {
			continue
		}
		if d.opener == "" && !standsAsSentence(right) {
			continue
		}
		// A noun phrase whose only verbs sit in relative clauses is a fragment: "A number that is true today".
		if !finiteOutsideRelative(syntax.Parse(checkMask(right), nil), 0) {
			continue
		}
		score := max(WordCount(left), WordCount(right))
		if bestScore < 0 || score < bestScore {
			best, bestScore = left+" "+right, score
		}
	}
	return best, bestScore >= 0
}

// coordinateDivision divides at a comma before a conjunction the clause pass
// left inside one clause, as in "X, so Y" or "X, but Y". It is the fallback
// behind the clause boundaries, and each half must read as a sentence.
func coordinateDivision(s *syntax.Sentence, source string) (string, bool) {
	best, bestScore := "", -1
	for _, d := range beforeCoordinate(s, source) {
		if cutsAside(mask(source), d.leftEnd, d.rightStart) {
			continue
		}
		left := strings.TrimRight(source[:d.leftEnd], " ,;:—–-") + "."
		right := capitalizeOpening(joinOpener(d.opener, restOf(source, d)))
		if WordCount(left) < minimumHalf || WordCount(right) < minimumHalf {
			continue
		}
		if w, ok := lastWordBefore(s, len(source[:d.leftEnd])); ok && danglingTags.Contains(w.Tag) {
			continue
		}
		// A comma before a conjunction is a real boundary, so the head stands even
		// where the whole sentence named no main clause for it.
		if n := wordFrom(s, d.rightStart); n >= 0 {
			if tag := s.Words[n].Tag; tag == "IN" || tag == "TO" {
				continue
			}
		}
		if !standsAsSentence(right) {
			continue
		}
		score := max(WordCount(left), WordCount(right))
		if bestScore < 0 || score < bestScore {
			best, bestScore = left+" "+right, score
		}
	}
	return best, bestScore >= 0
}

// Leading closes a sentence at a clause boundary the parser finds. It keeps the
// longest head that stands as a sentence under the cap, and reports false when
// no boundary gives one.
func Leading(sentence string) (string, bool) {
	masked := mask(sentence)
	s := syntax.Parse(masked, opaque(sentence, masked))
	best := ""
	for _, d := range divisions(s, sentence) {
		if !admissible(s, sentence, d) {
			continue
		}
		head := strings.TrimRight(sentence[:d.leftEnd], " ,;:—–-") + "."
		if n := WordCount(head); n >= minimumHalf && n <= SentenceWordCap && len(head) > len(best) {
			best = head
		}
	}
	return best, best != ""
}

// Cuts answers each byte offset where sentence can close at a clause boundary
// and leave a sentence: the boundaries the division repair admits.
func Cuts(sentence string) []int {
	masked := mask(sentence)
	s := syntax.Parse(masked, opaque(sentence, masked))
	var out []int
	for _, d := range divisions(s, sentence) {
		if admissible(s, sentence, d) {
			out = append(out, d.leftEnd)
		}
	}
	return out
}

// minimumHalf keeps a division from writing a sentence too short to stand alone.
const minimumHalf = 3

// withCloser writes closer in front of the stop that ends rest.
func withCloser(rest, closer string) string {
	if closer == "" {
		return rest
	}
	body := strings.TrimRight(rest, ".!? ")
	return body + " " + closer + rest[len(body):]
}

// objectRelative reports ", which X does not show" at the end of a sentence: a
// relative clause with a subject of its own whose verb ends the sentence. The
// relative word is the verb's object.
func objectRelative(s *syntax.Sentence, c syntax.Clause) bool {
	if c.Kind != syntax.Relative || !c.Comma || c.Depth != 1 || s.Words[c.Link].Lower() != "which" || c.Subject == nil || c.Verb == nil {
		return false
	}
	last := len(s.Words) - 1
	for last > 0 && punctuationTag(s.Words[last].Tag) {
		last--
	}
	return c.Verb.Last == last && c.Subject.First == c.Link+1 && !strings.HasPrefix(s.Words[last].Tag, "VBN")
}

func punctuationTag(tag string) bool { return tag == "." || tag == "," || tag == ":" }

// joinOpener puts the opener in front of the rest, and capitalizes what opens the sentence.
func joinOpener(opener, rest string) string {
	if opener != "" {
		rest = opener + " " + rest
	}
	return capitalizeOpening(rest)
}

// divisions answers each clause boundary where both sides stand as sentences.
func divisions(s *syntax.Sentence, source string) []division {
	var out []division
	for k := 1; k < len(s.Clauses); k++ {
		c := s.Clauses[k]
		main, ok := mainBefore(s, k)
		if !ok || c.Link < 0 || c.Link+1 >= len(s.Words) {
			continue
		}
		if c.Verb == nil && !(c.Kind == syntax.Punctuated && resumesAfter(s, c.Link+1)) {
			continue
		}
		// "X must default to blocked, never inherit Y": a bare verb after a comma shares the main clause's subject and modal, so both are named again rather than read as a command.
		if c.Verb != nil && c.Verb.Imperative && c.Subject == nil && main.Verb != nil && !main.Verb.Imperative && main.Subject != nil {
			opener := "It"
			if s.Plural(*main.Subject) {
				opener = "They"
			}
			for i := main.Verb.First; i <= main.Verb.Last; i++ {
				if s.Words[i].Tag == "MD" {
					opener += " " + s.Words[i].Lower()
					break
				}
			}
			out = append(out, division{
				leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
				rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
				opener:     opener,
			})
			continue
		}
		opener, ok := openerFor(s, c, main, source)
		if !ok {
			if d, ok := gerundDivision(s, c, main, source); ok {
				out = append(out, d)
				continue
			}
			if objectRelative(s, c) {
				out = append(out, division{
					leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
					rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
					closer:     "that",
				})
			}
			continue
		}
		d := division{
			leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
			rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
			opener:     opener,
		}
		if c.Kind != syntax.Relative && c.Kind != syntax.Subordinate {
			if i := participleComma(s, c); i > 0 && s.Words[i-1].Text != "," {
				d.comma = s.Words[i-1].End
			}
		}
		out = append(out, d)
	}
	out = append(out, beforeSubordinate(s, source)...)
	return out
}

// beforeCoordinate divides at ", <conjunction>" when the clause pass left both
// clauses as one. "X, so Y" and "X, but Y" where the parser read the whole as
// a single clause. Each side must stand as a sentence, which bestDivision
// checks. A list keeps its conjunction, because a comma before one inside a
// list does not open a clause of its own.
func beforeCoordinate(s *syntax.Sentence, source string) []division {
	var out []division
	for i := 2; i+1 < len(s.Words); i++ {
		connector, known := connectors[s.Words[i].Lower()]
		if !known || s.Words[i-1].Text != "," {
			continue
		}
		if closingClause(s, i) {
			continue
		}
		// A comma inside the clause before the conjunction ends a list, not a clause.
		if s.Words[i].Lower() != "so" && listBefore(s, syntax.Clause{Link: i}) {
			continue
		}
		out = append(out, division{
			leftEnd:    outsideSpans(source, s.Words[lastBefore(s, i)].End, true),
			rightStart: outsideSpans(source, s.Words[i+1].Start, false),
			opener:     connector,
		})
	}
	return out
}

// closingClause reports a conjunction that closes a subordinate clause before
// the sentence's own verb, as in "so that" or "and so", where no clause opens.
func closingClause(s *syntax.Sentence, i int) bool {
	w := s.Words[i].Lower()
	if w == "and" || w == "or" {
		return false
	}
	return i+1 < len(s.Words) && s.Words[i+1].Lower() == "that"
}

// beforeSubordinate divides at ", and" when a subordinate clause and then a main
// clause follow it, as in ", and if the cache is cold, the build waits".
func beforeSubordinate(s *syntax.Sentence, source string) []division {
	var out []division
	for k := 1; k+1 < len(s.Clauses); k++ {
		c, next := s.Clauses[k], s.Clauses[k+1]
		if c.Kind != syntax.Subordinate || next.Kind != syntax.Opens || next.Depth != 0 || next.Subject == nil || next.Verb == nil {
			continue
		}
		conj := c.Link - 1
		if conj > 0 && s.Words[conj].Tag == "IN" {
			// "for as long as": the preposition goes with the subordinate clause.
			conj--
		}
		if conj < 1 || s.Words[conj].Tag != "CC" || s.Words[conj-1].Text != "," {
			continue
		}
		connector, known := connectors[s.Words[conj].Lower()]
		if _, hasMain := mainBefore(s, k); !known || !hasMain {
			continue
		}
		out = append(out, division{
			leftEnd:    outsideSpans(source, s.Words[lastBefore(s, conj)].End, true),
			rightStart: outsideSpans(source, s.Words[conj+1].Start, false),
			opener:     connector,
		})
	}
	return out
}

// outsideSpans moves an offset out of a verbatim span, to its end or to its start.
// The parser reads a masked span, whose filler word stops short of the closing backtick.
func outsideSpans(source string, at int, toEnd bool) int {
	for _, span := range verbatimSpans(source) {
		if span[0] < at && at < span[1] {
			if toEnd {
				return span[1]
			}
			return span[0]
		}
	}
	return at
}

// mainBefore answers the main clause that clause k attaches to.
func mainBefore(s *syntax.Sentence, k int) (syntax.Clause, bool) {
	for j := k - 1; j >= 0; j-- {
		if c := s.Clauses[j]; c.Depth == 0 && c.Verb != nil {
			return c, true
		}
	}
	return syntax.Clause{}, false
}

// lastBefore answers the last word ahead of i that is not a comma or a dash.
func lastBefore(s *syntax.Sentence, i int) int {
	i--
	for i > 0 && strings.Contains(",—–--", s.Words[i].Text) {
		i--
	}
	return i
}

// finiteBetween reports a finite verb among words from up to end.
func finiteBetween(s *syntax.Sentence, from, end int) bool {
	for i := max(from, 0); i < end && i < len(s.Words); i++ {
		if finiteAt(s, i) {
			return true
		}
	}
	return false
}

// resumesAfter reports whether a main clause with a subject starting at word i
// resumes after a relative clause, as in ": a map that carries nothing is wasteful".
func resumesAfter(s *syntax.Sentence, i int) bool {
	for _, c := range s.Clauses {
		if c.Kind == syntax.Opens && c.Depth == 0 && c.Verb != nil && c.Subject != nil && c.Subject.First == i {
			return true
		}
	}
	return false
}

// closesTheSentence reports whether the clause and the clauses under it run to
// the end of the sentence. A clause the sentence returns from, as in "the file,
// which fails, is gone", cannot stand alone.
func closesTheSentence(s *syntax.Sentence, c syntax.Clause) bool {
	after := false
	for _, later := range s.Clauses {
		if later.First == c.First {
			after = true
			continue
		}
		if after && later.Depth < c.Depth && later.Kind != syntax.Coordinate {
			return false
		}
	}
	for i := c.Link + 1; i+1 < len(s.Words); i++ {
		if s.Words[i].Text == "," && strings.HasPrefix(s.Words[i+1].Tag, "VB") {
			return false
		}
	}
	return true
}

// opensWithCapital reports whether the word at i reads right with a capital.
// A name written in lower case, such as a command, does not. A code span does
// not either, whatever word the mask wrote over it.
func opensWithCapital(s *syntax.Sentence, i int, source string) bool {
	w := s.Words[i]
	if outsideSpans(source, w.Start+1, false) != w.Start+1 {
		return false
	}
	first, _ := utf8.DecodeRuneInString(w.Text)
	return !(unicode.IsLower(first) && (w.Tag == "NNP" || w.Tag == "NNPS"))
}

// restated names the main clause's subject again, for the verb group of c that
// shared it. A short subject repeats, and an indefinite article becomes the.
// Otherwise a pronoun stands in, chosen to agree with c's verb. It answers false
// when no pronoun agrees, as for a single person and a verb in -s.
func restated(s *syntax.Sentence, main, c syntax.Clause, source string) (string, bool) {
	if main.Subject == nil || main.Verb == nil || c.Verb == nil {
		return "", false
	}
	// A subject before an -ing form writes "It using hyperfine", which is no sentence.
	if s.Words[c.Verb.First].Tag == "VBG" || c.Link >= 0 && c.Link+1 < len(s.Words) && s.Words[c.Link+1].Tag == "VBG" {
		return "", false
	}
	subject := *main.Subject
	if subject.First > 0 && s.Words[subject.First-1].Tag == "VBG" {
		// The noun phrase is the object of a gerund, and the gerund is the subject: "freezing the HOW pins it".
		return "", false
	}
	// A subject the parser found after its verb, or one that opens on a bare noun, is a misreading.
	if subject.Last >= main.Verb.First || !opensSubject(s, subject.First) {
		return "", false
	}
	head := s.Words[subject.Head]
	if head.Tag == "PRP" {
		return head.Text, true
	}
	short := subject.Last-subject.First < restateLimit && !subject.Coordinated &&
		s.Words[subject.Last+1].Tag != "IN"
	if short && opensWithCapital(s, subject.First, source) {
		// A noun that disagrees with the verb is not that verb's subject: "its reads ... keeps".
		if !agrees(s, subject, *c.Verb) {
			return "", false
		}
		text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
		if subject.Det == subject.First {
			if det := s.Words[subject.Det].Lower(); det == "a" || det == "an" {
				text = "the" + text[len(det):]
			}
		}
		return text, true
	}
	plural := s.Plural(subject)
	switch s.Words[c.Verb.Head].Tag {
	case "VBZ":
		plural = false
	case "VBP":
		plural = true
	}
	switch {
	case plural:
		return "they", true
	case s.Person(subject):
		return "", false
	}
	return "it", true
}

// spliced reports whether a comma the splice pattern matched joins main clauses.
func spliced(prose string, loc []int) bool {
	if loc[2] >= 0 {
		return joinsClauses(prose, loc[2])
	}
	return isClause(clauseBefore(prose, loc[0])) && endsMainClause(prose, loc[0])
}

// endsMainClause reports whether a main clause runs up to the comma at byte
// at. "If the cache is cold," and "where each run gets a machine," do not.
func endsMainClause(prose string, at int) bool {
	for _, span := range sentenceSpans(prose) {
		if at < span[0] || at >= span[1] {
			continue
		}
		text := prose[span[0]:span[1]]
		s := syntax.Parse(text, opaque(text, text))
		last := -1
		for i, w := range s.Words {
			if w.End <= at-span[0] {
				last = i
			}
		}
		for _, c := range s.Clauses {
			if c.First <= last && last <= c.Last {
				return c.Depth == 0 && c.Verb != nil && standsAlone(s, 0, last+1)
			}
		}
		return false
	}
	return false
}

// joinsClauses reports whether the conjunction at byte at joins a main clause
// to another that names its own subject. The end of a list does not.
func joinsClauses(prose string, at int) bool {
	word, _, _ := strings.Cut(prose[at:], " ")
	if !syntax.Is(word, "coordinator") {
		// "then" is an adverb to the parser, and the splice pattern alone judges it.
		return true
	}
	for _, span := range sentenceSpans(prose) {
		if at < span[0] || at >= span[1] {
			continue
		}
		s := syntax.Parse(prose[span[0]:span[1]], opaque(prose[span[0]:span[1]], prose[span[0]:span[1]]))
		for k, c := range s.Clauses {
			if c.Link >= 0 && s.Words[c.Link].Start == at-span[0] {
				if c.Kind != syntax.Coordinate || c.Depth != 0 || c.Subject == nil || listBefore(s, c) {
					return false
				}
				// Each half must be a sentence: a main clause before, and a subject that
				// agrees with its verb after.
				if k == 0 || s.Clauses[k-1].Verb == nil || !standsAlone(s, 0, c.Link) || !subjectFollows(s, c) || !agrees(s, *c.Subject, *c.Verb) {
					return false
				}
				// A so after an instruction states its purpose, and joins nothing a period can take.
				main, ok := mainBefore(s, k)
				return !strings.EqualFold(word, "so") || ok && !main.Verb.Imperative && !instructs(s)
			}
		}
		return false
	}
	return false
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

// clauseStart answers the earliest word of the clause that ends before word i.
func clauseStart(s *syntax.Sentence, i int) int {
	start := 0
	for _, c := range s.Clauses {
		if c.First < i && c.First > start && c.Link != i {
			start = c.First
		}
	}
	return start
}

const restateLimit = 4
