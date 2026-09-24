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
// The and of a list of actions carries no meaning a period loses.
var connectors = map[string]string{
	"and": "",
	"but": "However,",
	"yet": "However,",
	"so":  "As a result,",
}

// fixSentenceCap divides every over-cap sentence where its grammar allows. A
// sentence with no clause boundary to divide at stays whole, and Check reports it.
func fixSentenceCap(prose string) string {
	for range len(strings.Fields(prose)) + 1 {
		next, divided := divideNext(prose)
		if !divided {
			return prose
		}
		prose = next
	}
	return prose
}

// divideNext divides the earliest over-cap sentence that a clause boundary can divide.
func divideNext(prose string) (string, bool) {
	masked := mask(prose)
	off := opaque(prose, masked)
	for _, span := range sentenceSpans(prose) {
		start, end := span[0], span[1]
		if WordCount(masked[start:end]) <= SentenceWordCap {
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
		left := strings.TrimRight(source[:d.leftEnd], " ,") + "."
		right := joinOpener(d.opener, source[d.rightStart:])
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
		if !ok || c.Verb == nil || c.Link < 0 || c.Link+1 >= len(s.Words) {
			continue
		}
		opener, ok := openerFor(s, c, main, source)
		if !ok {
			continue
		}
		out = append(out, division{
			leftEnd:    s.Words[lastBefore(s, c.Link)].End,
			rightStart: s.Words[c.Link+1].Start,
			opener:     opener,
		})
	}
	return out
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

// lastBefore answers the last word ahead of i that is not a comma.
func lastBefore(s *syntax.Sentence, i int) int {
	i--
	for i > 0 && s.Words[i].Text == "," {
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
		if c.Subject != nil {
			return connector, opensWithCapital(s, c.Link+1)
		}
		if main.Verb.Imperative {
			return connector, opensSentence(s, main)
		}
		if main.Subject == nil {
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
	case syntax.Subordinate:
		if link != "because" || !closesTheSentence(s, c) {
			return "", false
		}
		return "This is because", true
	}
	return "", false
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
// A name written in lower case, such as a command, does not.
func opensWithCapital(s *syntax.Sentence, i int) bool {
	w := s.Words[i]
	first, _ := utf8.DecodeRuneInString(w.Text)
	return !(unicode.IsLower(first) && (w.Tag == "NNP" || w.Tag == "NNPS"))
}

// restated names the main clause's subject again, for the verb group of c that
// shared it. A short subject repeats, and an indefinite article becomes the.
// Otherwise a pronoun stands in, chosen to agree with c's verb. It answers false
// when no pronoun agrees, as for a single person and a verb in -s.
func restated(s *syntax.Sentence, main, c syntax.Clause, source string) (string, bool) {
	subject := *main.Subject
	head := s.Words[subject.Head]
	if head.Tag == "PRP" {
		return head.Text, true
	}
	short := subject.Last-subject.First < restateLimit && !subject.Coordinated &&
		s.Words[subject.Last+1].Tag != "IN"
	if short && opensWithCapital(s, subject.First) {
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

// listBefore reports a comma inside the clause before c, which makes ", and"
// the end of a list rather than a join between clauses.
func listBefore(s *syntax.Sentence, c syntax.Clause) bool {
	for i := c.Link - 2; i >= 0 && i >= clauseStart(s, c.Link); i-- {
		if s.Words[i].Text == "," {
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

// opensSentence reports whether a clause starts at the sentence's earliest
// word, which an imperative has to: "Write the file".
func opensSentence(s *syntax.Sentence, c syntax.Clause) bool {
	for i := 0; i < c.Verb.First; i++ {
		if s.Words[i].Tag != "RB" && s.Words[i].Tag != "``" {
			return false
		}
	}
	return true
}

const restateLimit = 4
