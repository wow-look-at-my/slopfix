package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// FixSemicolonByHand is the Fix text of a semicolon that no period can replace.
const FixSemicolonByHand = "Rewrite it by hand. The semicolon joins items or phrases, and a period leaves a fragment on one side."

// ByHand reports whether a Fix text asks for a rewrite by hand, which no repair writes.
func ByHand(fix string) bool { return strings.HasPrefix(fix, "Rewrite it by hand") }

// semicolonJoins answers the offset of each semicolon of masked that a period
// can replace. A semicolon divides only where the words before it hold a main
// clause and the words after it open one.
func semicolonJoins(masked string) []int {
	var joins []int
	for _, span := range sentenceSpans(masked) {
		text := masked[span[0]:span[1]]
		if !strings.Contains(text, ";") {
			continue
		}
		s := syntax.Parse(text, nil)
		from := 0
		for i, w := range s.Words {
			if w.Text != ";" {
				continue
			}
			if !semicolonDivides(s, from, i) {
				continue
			}
			joins = append(joins, span[0]+w.Start)
			from = i + 1
		}
	}
	return joins
}

// semicolonDivides reports whether the semicolon at word i of s ends a sentence
// that opens at word from, and the words after it open a sentence of their own.
// A semicolon that ends the sentence always divides.
func semicolonDivides(s *syntax.Sentence, from, i int) bool {
	rest := i + 1
	for rest < len(s.Words) && strings.IndexFunc(s.Words[rest].Text, unicode.IsLetter) < 0 {
		rest++
	}
	if rest == len(s.Words) {
		return true
	}
	return standsAlone(s, from, i) && opensSentenceAt(s, i)
}

// opensSentenceAt reports whether the words after the mark at word i open with
// their own subject and a verb that agrees with it, with an imperative, or with
// a subordinate clause and then its main clause.
func opensSentenceAt(s *syntax.Sentence, i int) bool {
	for k, c := range s.Clauses {
		if c.Depth == 0 && c.Link == i {
			if c.Subject != nil {
				return subjectFollows(s, c) && agrees(s, *c.Subject, *c.Verb)
			}
			if resumesAfter(s, i+1) && opensSubject(s, i+1) {
				return true
			}
			// The tagger reads a bare verb after the mark as a noun, so the words are read with the subject an imperative leaves out.
			if c.Verb != nil && verbFollows(s, c) && imperativeTag(s, *c.Verb) {
				return true
			}
			// A finite verb right after the first word makes that word its subject: "; deliverable is prose".
			return !finiteAt(s, i+2) && opensImperative(s.Text[s.Words[i+1].Start:])
		}
		// "; when the cache is cold, the build waits": the subordinator took the link.
		if c.Kind == syntax.Subordinate && c.Link == i+1 {
			if k+1 < len(s.Clauses) {
				if next := s.Clauses[k+1]; next.Kind == syntax.Opens && next.Depth == 0 && next.Subject != nil && next.Verb != nil {
					return true
				}
			}
			// "; when the gap is fixed, return the verdict": the parser keeps an imperative in the subordinate clause.
			comma := firstCommaFrom(s, i+1)
			return comma > 0 && comma+1 < len(s.Words) && (imperativeTag(s, syntax.Phrase{First: comma + 1, Last: comma + 1}) || opensImperative(s.Text[s.Words[comma+1].Start:]))
		}
	}
	return false
}

// finiteAt reports a finite verb at word i of s.
func finiteAt(s *syntax.Sentence, i int) bool {
	if i < 0 || i >= len(s.Words) {
		return false
	}
	switch s.Words[i].Tag {
	case "VBZ", "VBP", "VBD", "MD":
		return true
	}
	return false
}

// imperativeTag reports a verb group that leads with a bare verb. The tagger
// often reads a bare verb with no subject as present tense.
func imperativeTag(s *syntax.Sentence, verb syntax.Phrase) bool {
	for j := verb.First; j <= verb.Last; j++ {
		switch s.Words[j].Tag {
		case "VB", "VBP":
			return true
		case "RB":
			continue
		}
		return false
	}
	return false
}

// fixSemicolons writes a period for each semicolon that semicolonJoins admits.
// The blank after it is read from the text, because the mask writes a blank
// over the backtick of a code span.
func fixSemicolons(text string) string {
	starts := semicolonJoins(checkMask(text))
	var joins [][]int
	for _, at := range starts {
		joins = append(joins, []int{at, at + len(semicolonRun.FindString(text[at:]))})
	}
	return breakWith(text, joins, nil)
}
