package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

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
