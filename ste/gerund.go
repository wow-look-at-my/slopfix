package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// frontedDivision divides at a conjunction that a fronted clause follows, as
// in "X but, when Y is set, benchmarks Z". The verb after the fronted clause
// shares the subject of X, so the rest names it again: "X. When Y is set, it
// benchmarks Z".
func frontedDivision(s *syntax.Sentence, source string) (string, bool) {
	for i := 1; i+3 < len(s.Words); i++ {
		_, ok := connectors[s.Words[i].Lower()]
		if !ok || s.Words[i].Tag != "CC" || s.Words[i+1].Text != "," || !syntax.Is(s.Words[i+2].Text, "subordinator") {
			continue
		}
		comma := -1
		for j := i + 3; j < len(s.Words) && comma < 0; j++ {
			if s.Words[j].Text == "," {
				comma = j
			}
		}
		if comma < 0 || !standsAlone(s, 0, i) {
			continue
		}
		pronoun := ""
		switch {
		case comma+1 >= len(s.Words):
		case s.Words[comma+1].Tag == "VBZ":
			pronoun = "it"
		case s.Words[comma+1].Tag == "VBP":
			pronoun = "they"
		}
		if pronoun == "" {
			continue
		}
		// A masked code span's word stops short of its backticks, so each offset moves out of the span.
		opens := outsideSpans(source, s.Words[i+2].Start, false)
		closes := outsideSpans(source, s.Words[comma].End, true)
		resumes := outsideSpans(source, s.Words[comma+1].Start, false)
		rest := source[opens:closes] + " " + pronoun + " " + source[resumes:]
		left := strings.TrimRight(source[:outsideSpans(source, s.Words[i-1].End, true)], " ,") + "."
		// The conjunction drops, because the reorder repair moves a sentence adverb such as "However," to the end of the sentence.
		return left + " " + joinOpener("", rest), true
	}
	return "", false
}

// gerundDivision divides before an -ing form after a dash. The form says how
// the main verb acts, and the rest names a pronoun before the form as a
// finite verb: "benchmarks X -- using hyperfine" becomes "benchmarks X. It
// uses hyperfine". The pronoun and the verb agree with the main verb.
func gerundDivision(s *syntax.Sentence, c, main syntax.Clause, source string) (division, bool) {
	at := c.Link + 1
	if c.Kind != syntax.Punctuated || c.Depth != 0 || !dashMark(s.Words[c.Link].Text) || at+1 >= len(s.Words) ||
		s.Words[at].Tag != "VBG" || main.Verb == nil {
		return division{}, false
	}
	word := s.Words[at]
	if outsideSpans(source, word.Start, false) != word.Start {
		return division{}, false
	}
	// A gerund whose phrase runs to a finite verb with no comma between is the subject of that verb: "-- keeping it here lets X".
	for k := at + 1; k < len(s.Words) && s.Words[k].Text != ","; k++ {
		if finiteAt(s, k) {
			return division{}, false
		}
	}
	// The clause that names the subject sets the person. A later clause can share that subject.
	for _, opening := range s.Clauses {
		if opening.First >= c.First {
			break
		}
		if opening.Depth == 0 && opening.Verb != nil && opening.Subject != nil {
			main = opening
			break
		}
	}
	pronoun, plural := "", false
	switch s.Words[main.Verb.Head].Tag {
	case "VBZ":
		pronoun = "It"
	case "VBP":
		pronoun, plural = "They", true
	default:
		return division{}, false
	}
	return division{
		leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
		rightStart: s.Words[at+1].Start,
		opener:     pronoun + " " + presentOf(baseFromGerund(source[word.Start:word.End]), plural),
	}, true
}
