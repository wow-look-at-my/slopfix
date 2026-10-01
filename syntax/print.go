package syntax

import (
	"fmt"
	"strings"
)

// Brackets writes the sentence with each noun phrase in [ ] and each verb group in < >.
func (s *Sentence) Brackets() string {
	var out []string
	for i := 0; i < len(s.Words); i++ {
		if ph, ok := s.PhraseAt(i); ok && ph.First == i {
			open, shut := "[", "]"
			if ph.Kind == VerbGroup {
				open, shut = "<", ">"
			}
			out = append(out, open+s.Span(ph.First, ph.Last)+shut)
			i = ph.Last
			continue
		}
		out = append(out, s.Words[i].Text)
	}
	return strings.Join(out, " ")
}

var kindNames = map[LinkKind]string{
	Opens: "opens", Coordinate: "coordinate", Subordinate: "subordinate",
	Relative: "relative", Punctuated: "punctuated",
}

// Outline writes each clause on a line of its own: its link, depth, subject, verb and text.
func (s *Sentence) Outline() string {
	var lines []string
	for _, c := range s.Clauses {
		subject, verb := "-", "-"
		if c.Subject != nil {
			subject = s.Span(c.Subject.First, c.Subject.Last)
		}
		if c.Verb != nil {
			verb = s.Span(c.Verb.First, c.Verb.Last)
		}
		lines = append(lines, fmt.Sprintf("%s/%d subject=%q verb=%q: %s",
			kindNames[c.Kind], c.Depth, subject, verb, s.Span(c.First, c.Last)))
	}
	return strings.Join(lines, "\n")
}

// Tags writes each word with its tag, as word/TAG.
func (s *Sentence) Tags() string {
	var out []string
	for _, w := range s.Words {
		out = append(out, w.Text+"/"+w.Tag)
	}
	return strings.Join(out, " ")
}
