package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// A division ends the left sentence at leftEnd. The right sentence opens with
// opener, then the source from rightStart.
type division struct {
	leftEnd, rightStart int
	opener              string
}

// connectors map a conjunction to the words that open the sentence after it.
// A period says what and and so say, so both drop.
var connectors = map[string]string{
	"and": "",
	"but": "However,",
	"yet": "However,",
	"so":  "",
}

// FixByHand is the Fix text of a long sentence that no division can repair.
const FixByHand = "Rewrite it by hand as shorter sentences. No division keeps each half a grammatical sentence."

// fixSentenceCap divides every over-cap sentence where both halves stay
// grammatical sentences. It first tries the clause boundaries, then the word
// boundaries.
func fixSentenceCap(prose string, d division) string {
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
		if rewritten, ok := bestDivision(s, prose[start:end]); ok {
			return prose[:start] + rewritten + prose[end:], true
		}
	}
	return prose, false
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
		left := strings.TrimRight(source[:d.leftEnd], " ,;:—–-") + "."
		right := bold + joinOpener(d.opener, source[d.rightStart:])
		if WordCount(left) < minimumHalf || WordCount(right) < minimumHalf {
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

// joinOpener puts the opener in front of the rest, and capitalizes what opens the sentence.
func joinOpener(opener, rest string) string {
	if opener != "" {
		rest = opener + " " + rest
	}
	return capitalizeOpening(rest)
}

// capitalizeOpening capitalizes a lower-case letter at the start, and leaves anything else.
func capitalizeOpening(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if !unicode.IsLower(first) {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
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
		opener, ok := openerFor(s, c, main, source)
		if !ok {
			continue
		}
		out = append(out, division{
			leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
			rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
			opener:     opener,
		})
	}
	return append(out, beforeSubordinate(s, source)...)
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
	for _, span := range verbatimSpan.FindAllStringIndex(source, -1) {
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

// openerFor answers the words that open the clause as a sentence of its own,
// and whether the clause can stand as a sentence at all.
func openerFor(s *syntax.Sentence, c, main syntax.Clause, source string) (string, bool) {
	link := s.Words[c.Link].Lower()
	switch c.Kind {
	case syntax.Coordinate:
		if c.Depth != 0 {
			return "", false
		}
		connector, ok := connectors[link]
		if !ok || c.Comma && listBefore(s, c) {
			return "", false
		}
		if link == "so" && (!c.Comma || c.Subject == nil || main.Verb.Imperative || instructs(s)) {
			// A so after an instruction, or with no comma, states a purpose. A so with no subject leaves no clause.
			return "", false
		}
		// The clause before the link needs a verb, or the conjunction joins noun phrases.
		if s.Clauses[indexOf(s, c)-1].Verb == nil {
			return "", false
		}
		if c.Subject != nil {
			return connector, subjectFollows(s, c) && agrees(s, *c.Subject, *c.Verb) && opensWithCapital(s, c.Link+1, source)
		}
		// A shared subject needs its verb right after the link, and no aside before the link.
		if !verbFollows(s, c) || c.Link > 0 && strings.Contains("—–--", s.Words[c.Link-1].Text) || laterVerb(s, c) {
			return "", false
		}
		if main.Verb.Imperative {
			return connector, startsTheSentence(s, main)
		}
		// The subject to name again belongs to the clause right before the link.
		if main.Subject == nil || s.Clauses[indexOf(s, c)-1].First != main.First || finiteBetween(s, main.Verb.Last+1, c.Link) {
			return "", false
		}
		subject, ok := restated(s, main, c, source)
		return strings.TrimSpace(connector + " " + subject), ok
	case syntax.Relative:
		if link != "which" || !c.Comma || c.Subject != nil || !closesTheSentence(s, c) {
			return "", false
		}
		if s.Words[c.Verb.Head].Tag == "VBP" {
			return "These", true
		}
		return "This", true
	case syntax.Punctuated:
		// A colon or a dash before a clause that names its own subject ends a sentence.
		if c.Depth != 0 || c.Subject == nil && !resumesAfter(s, c.Link+1) {
			return "", false
		}
		if c.Subject != nil && !subjectFollows(s, c) || !opensSubject(s, c.Link+1) {
			return "", false
		}
		return "", opensWithCapital(s, c.Link+1, source)
	case syntax.Subordinate:
		if link != "because" || !closesTheSentence(s, c) {
			return "", false
		}
		return "This is because", true
	}
	return "", false
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
