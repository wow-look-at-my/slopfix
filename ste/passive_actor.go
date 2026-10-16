package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// rewriteActor writes the active voice for a passive clause that names its
// actor: "The file is read by the gate" becomes "The gate reads the file".
func rewriteActor(s *syntax.Sentence, source string) (string, bool) {
	last := lastContent(s)
	i, j := auxiliary(s, 1, beForms)
	if i < 1 || j < 0 || s.Words[j].Tag != "VBN" {
		return "", false
	}
	if !simpleSpan(s, i, j) {
		return "", false
	}
	// The actor is the phrase after "by", and it must close the clause.
	by := -1
	for k := j + 1; k < last; k++ {
		if s.Words[k].Lower() == "by" {
			by = k
			break
		}
	}
	if by < 0 || by+1 > last {
		return "", false
	}
	end := last
	for k := by + 1; k <= last; k++ {
		if endsActor(s.Words[k]) {
			end = k - 1
			break
		}
	}
	if end < by+1 {
		return "", false
	}
	// "by" and a gerund name the means, and no actor: "is asserted by pairing it with X".
	if s.Words[by+1].Tag == "VBG" {
		return "", false
	}
	// A second verb group after the actor shares the passive subject, which the rewrite moves away from it: "is untouched by this and is now the slowest".
	for k := by + 2; k < end; k++ {
		if s.Words[k].Tag == "CC" && (strings.HasPrefix(s.Words[k+1].Tag, "VB") || s.Words[k+1].Tag == "MD") {
			return "", false
		}
	}
	first, ok := subjectStart(s, i)
	if !ok {
		return "", false
	}
	// A bare "be" is active only behind a modal: "X must be read by Y" becomes "Y must read X".
	subjectEnd, verb := i, ""
	if s.Words[i].Lower() == "be" {
		if i-1 <= first || s.Words[i-1].Tag != "MD" {
			return "", false
		}
		subjectEnd = i - 1
		verb = source[s.Words[i-1].Start:s.Words[i-1].End] + " " + baseFromParticiple(s.Words[j].Lower())
	} else {
		// "has been read" and "must not be read" carry a word no rewrite here moves.
		if before := s.Words[i-1]; before.Tag == "MD" || haveForms.Contains(before.Lower()) || beForms.Contains(before.Lower()) {
			return "", false
		}
		verb = passiveVerb(s.Words[j].Lower(), s.Words[i].Lower(), s.Plural(syntax.Phrase{Head: end}))
	}
	from, to := s.Words[first].Start, s.Words[end].End
	subject := strings.TrimSpace(unbold(source[from:s.Words[subjectEnd].Start]))
	actor := strings.TrimSpace(unbold(source[s.Words[by+1].Start:to]))
	if subject == "" || actor == "" || verb == "" {
		return "", false
	}
	object := subject
	if first == 0 {
		actor = capitalizeOpening(actor)
		object = lowerOpeningFor(subject, s.Words[0].Tag)
	}
	clause := rebold(source, from, to, actor+" "+verb+" "+object)
	return source[:from] + clause + source[to:], true
}

// endsActor reports a word the actor of a passive clause stops in front of.
// Punctuation ends it, and so does a subordinator.
func endsActor(w syntax.Word) bool {
	switch w.Text {
	case ",", ";", ":", "(":
		return true
	}
	if strings.Trim(w.Text, "-–—") == "" {
		return true
	}
	return syntax.Is(w.Lower(), "subordinator")
}

// subjectStart answers the first word of the subject in front of the verb
// group at i. An opening phrase a comma closes stays in front of the clause, as
// does the coordinator or subordinator after that comma. A relative pronoun
// stands for a noun outside the clause, so it has no subject to move.
func subjectStart(s *syntax.Sentence, i int) (int, bool) {
	first := 0
	for k := 0; k < i; k++ {
		if s.Words[k].Text == "," && leadsClause(s, first, k) {
			first = k + 1
		}
	}
	for first < i && (s.Words[first].Tag == "CC" || syntax.Is(s.Words[first].Lower(), "subordinator") || s.Words[first].Lower() == "so") {
		first++
	}
	// A subject opens on a noun phrase. A verb there is a reduced clause of the words before: "run by the action the same way X is".
	if first >= i || strings.HasPrefix(s.Words[first].Tag, "W") || strings.HasPrefix(s.Words[first].Tag, "VB") {
		return 0, false
	}
	return first, true
}

// leadsClause reports whether the words from first up to the comma at k stand
// in front of a clause rather than in a list. They carry a finite verb, or they
// open on a preposition or an adverb, or the comma is followed by a coordinator
// or a subordinator.
func leadsClause(s *syntax.Sentence, first, k int) bool {
	if k+1 < len(s.Words) {
		next := s.Words[k+1]
		if next.Tag == "CC" || next.Lower() == "so" || syntax.Is(next.Lower(), "subordinator") {
			return true
		}
	}
	if first < k {
		switch s.Words[first].Tag {
		case "IN", "RB", "TO", "VBG":
			return true
		}
	}
	for _, w := range s.Words[first:k] {
		switch w.Tag {
		case "VBZ", "VBP", "VBD", "MD":
			return true
		}
	}
	return false
}

// unbold drops the bold markers from a piece a rewrite moves.
func unbold(piece string) string {
	return strings.ReplaceAll(piece, "**", "")
}

// rebold writes the bold markers source carries between from and to around the
// rewritten clause. A run that opens inside the clause opens at its start, and
// a run that closes inside it closes at its end.
func rebold(source string, from, to int, clause string) string {
	inside := strings.Count(source[from:to], "**")
	if inside == 0 {
		return clause
	}
	startBold := strings.Count(source[:from], "**")%2 == 1
	endBold := (strings.Count(source[:from], "**")+inside)%2 == 1
	if !startBold {
		clause = "**" + clause
	}
	if !endBold {
		clause += "**"
	}
	return clause
}
