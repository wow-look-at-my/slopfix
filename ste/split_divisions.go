// split_divisions.go lists the places a sentence divides and the opener each second half takes.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

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
		// No list ends on "so", so a list before it does not swallow the clause.
		if !ok || c.Comma && link != "so" && listBefore(s, c) {
			return "", false
		}
		if link == "so" && (!c.Comma || c.Subject == nil || main.Verb.Imperative || instructs(s)) {
			// A so after an instruction, or with no comma, states a purpose. A so with no subject leaves no clause.
			return "", false
		}
		// The clause before the link needs a verb, or the conjunction joins noun
		// phrases.
		if s.Clauses[indexOf(s, c)-1].Verb == nil && !(c.Comma && c.Subject != nil && subjectFollows(s, c)) {
			return "", false
		}
		if p := c.Link + 1; c.Subject == nil && c.Verb != nil && c.Verb.First == p && p+2 < len(s.Words) &&
			(s.Words[p].Tag == "VBD" || s.Words[p].Tag == "VBN") && s.Words[p+1].Tag == "PRP" && finiteAt(s, p+2) {
			// "and dropped it leaves": a participle and its object open a clause whose own subject follows.
			return connector, opensWithCapital(s, p, source)
		}
		if c.Subject != nil {
			return connector, subjectFollows(s, c) && agrees(s, *c.Subject, *c.Verb) && opensWithCapital(s, c.Link+1, source)
		}
		// A shared subject needs its verb right after the link, and no aside before the link.
		if !verbFollows(s, c) || c.Link > 0 && strings.Contains("—–--", s.Words[c.Link-1].Text) || laterVerb(s, c) {
			return "", false
		}
		// A comma before the link puts the shared verb group in a list that the
		// conjunction continues: "fails to compile, fails its tests, or errors".
		if listsVerbs(source[:s.Words[c.Link].Start]) {
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
		if c.Depth != 0 {
			return "", false
		}
		if c.Subject == nil && !resumesAfter(s, c.Link+1) {
			if c.Verb == nil {
				return "", false
			}
			// A colon opens the clause that explains the head: "Stop: runs in
			// stop-gate mode ..." divides as "Stop. It runs in stop-gate mode".
			if s.Words[c.Link].Text == ":" && finiteBefore(s, c.Link+1, "") {
				// A determiner opens a subject of its own that the parse missed: "together: a line fix wrote X reads as Y".
				if tag := s.Words[c.Link+1].Tag; tag == "DT" || tag == "PRP$" || tag == "CD" {
					return "", opensWithCapital(s, c.Link+1, source)
				}
				if main.Subject != nil {
					if subject, ok := restated(s, main, c, source); ok {
						return subject, true
					}
				}
				return "", opensWithCapital(s, c.Link+1, source)
			}
			// A dash before a verb group leaves the clause before it whole, and the
			// subject the sentence opened with is named.
			if !dashMark(s.Words[c.Link].Text) {
				return "", false
			}
			if opensImperative(checkMask(source[s.Words[c.Link+1].Start:])) {
				return "", opensWithCapital(s, c.Link+1, source)
			}
			if main.Subject == nil {
				return "", false
			}
			subject, ok := restated(s, main, c, source)
			if !ok {
				return "", false
			}
			return subject, true
		}
		if c.Subject != nil && !subjectFollows(s, c) || !opensSubject(s, c.Link+1) {
			// "not prose: left in the text a rewrite wraps it": a participle phrase opens a clause that names its own subject later.
			if c.Subject != nil && s.Words[c.Link].Text == ":" && s.Words[c.Link+1].Tag == "VBN" {
				return "", opensWithCapital(s, c.Link+1, source) && c.Verb != nil && c.Subject.First > c.Link+1 && c.Subject.Last < c.Verb.First && finiteAt(s, c.Verb.First)
			}
			return "", false
		}
		return "", opensWithCapital(s, c.Link+1, source)
	case syntax.Subordinate:
		// ", and returns Y" after the reason is the main clause again, so the reason does not close the sentence.
		if link != "because" || !closesTheSentence(s, c) || coordinatedVerb(s, c.Link) {
			return "", false
		}
		return "This is because", true
	}
	return "", false
}
