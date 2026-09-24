package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

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
	off := offLimits(prose, masked)
	at := 0
	for _, sentence := range Sentences(masked) {
		start := strings.Index(masked[at:], sentence)
		if start < 0 {
			break
		}
		start += at
		end := start + len(sentence)
		at = end
		if WordCount(sentence) <= SentenceWordCap {
			continue
		}
		s := syntax.Parse(sentence, shift(off, -start))
		if rewritten, ok := bestDivision(s, prose[start:end]); ok {
			return prose[:start] + rewritten + prose[end:], true
		}
	}
	return prose, false
}

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
		if !ok {
			return "", false
		}
		if c.Subject != nil {
			return connector, opensWithCapital(s, c.Link+1)
		}
		if main.Verb.Imperative {
			return connector, true
		}
		if main.Subject == nil {
			return "", false
		}
		return strings.TrimSpace(connector + " " + restated(s, *main.Subject, source)), true
	case syntax.Relative:
		if link != "which" || !c.Comma || c.Depth != 1 || c.Subject != nil {
			return "", false
		}
		if s.Words[c.Verb.Head].Tag == "VBP" {
			return "These", true
		}
		return "This", true
	case syntax.Subordinate:
		if link != "because" || c.Depth != 1 {
			return "", false
		}
		return "This is because", true
	}
	return "", false
}

// opensWithCapital reports whether the word at i reads right with a capital.
// A name written in lower case, such as a command, does not.
func opensWithCapital(s *syntax.Sentence, i int) bool {
	w := s.Words[i]
	first, _ := utf8.DecodeRuneInString(w.Text)
	return !(unicode.IsLower(first) && (w.Tag == "NNP" || w.Tag == "NNPS"))
}

// restated names the subject again for a verb group that shared it. A short
// subject repeats, and an indefinite article becomes the. A long subject, or a
// name in lower case, becomes a pronoun.
func restated(s *syntax.Sentence, subject syntax.Phrase, source string) string {
	head := s.Words[subject.Head]
	if head.Tag == "PRP" {
		return head.Text
	}
	short := subject.Last-subject.First < restateLimit
	if short && opensWithCapital(s, subject.First) {
		text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
		if subject.Det == subject.First {
			if det := s.Words[subject.Det].Lower(); det == "a" || det == "an" {
				text = "the" + text[len(det):]
			}
		}
		return text
	}
	if s.Plural(subject) || s.Person(subject) {
		return "they"
	}
	return "it"
}

// restateLimit is how many words a subject may carry and still repeat.
const restateLimit = 4
