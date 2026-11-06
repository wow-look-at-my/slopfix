// split_divisions.go lists the places a sentence divides and the opener each second half takes.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// sharedModal answers the modal a bare verb after a comma shares with the main
// clause, as in "X must default to blocked, refuse the open default". It
// answers "" where the clause names its own subject or the main clause holds
// no modal, because "They refuse the default" has no verb to open on.
func sharedModal(s *syntax.Sentence, c, main syntax.Clause) string {
	if c.Verb == nil || !c.Verb.Imperative || c.Subject != nil || main.Verb == nil || main.Verb.Imperative || main.Subject == nil {
		return ""
	}
	for i := main.Verb.First; i <= main.Verb.Last; i++ {
		if s.Words[i].Tag == "MD" {
			return s.Words[i].Lower()
		}
	}
	return ""
}

// divisions answers each clause boundary where both sides stand as sentences.
func divisions(s *syntax.Sentence, source string) []division {
	var out []division
	dashed := dashAsides(source)
	for k := 1; k < len(s.Clauses); k++ {
		c := s.Clauses[k]
		main, ok := mainBefore(s, k)
		if !ok || c.Link < 0 || c.Link+1 >= len(s.Words) {
			continue
		}
		// A division inside a pair of dashes, or at either dash, halves the aside, unless the rest names its subject again.
		inAside := insideAny(dashed, s.Words[c.Link].Start)
		// The part before a division needs a verb with tense: "so newly created files." has none.
		if main.Verb == nil || (!main.Verb.Finite && !main.Verb.Imperative) {
			continue
		}
		if c.Verb == nil && !(c.Kind == syntax.Punctuated && resumesAfter(s, c.Link+1)) {
			continue
		}
		// "X must default to blocked, never inherit Y": a bare verb after a comma shares the main clause's subject and modal, so both are named again rather than read as a command. A pronoun with no modal to name opens no sentence, so that boundary is left to the clause boundary itself.
		if modal := sharedModal(s, c, main); modal != "" {
			opener := "It"
			if s.Plural(*main.Subject) {
				opener = "They"
			}
			out = append(out, division{
				leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
				rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
				opener:     opener + " " + modal,
			})
			continue
		}
		opener, ok := openerFor(s, c, main, source)
		if !ok {
			if d, ok := gerundDivision(s, c, main, source); ok {
				out = append(out, d)
				continue
			}
			if !inAside && objectRelative(s, c) {
				out = append(out, division{
					leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
					rightStart: outsideSpans(source, s.Words[c.Link+1].Start, false),
					closer:     "that",
				})
			}
			continue
		}
		if inAside && opener == "" {
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
