// The actor is the phrase after "by", and it must close the clause.

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
	// A participle between the passive verb and "by" governs that phrase.
	if byBelongsToOther(s, j, by) {
		return "", false
	}
	// "by" and a gerund name the means, which acts as a singular subject: "is asserted by pairing it with X" becomes "Pairing it with X asserts".
	means := s.Words[by+1].Tag == "VBG"
	// A bare noun after "by" names a manner, and a participle that is no verb has no active form.
	if !participleOfVerb(s.Words[j].Lower()) || !means && !(passiveClause{s: s, by: by}).namesActor() {
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
		// The adverbs between the modal and "be" stay with the modal: "must never be lost" becomes "must never lose".
		modal := i - 1
		for modal > first && (s.Words[modal].Tag == "RB" || s.Words[modal].Lower() == "not") {
			modal--
		}
		if modal <= first || s.Words[modal].Tag != "MD" {
			return "", false
		}
		subjectEnd = modal
		verb = source[s.Words[modal].Start:s.Words[i-1].End] + " " + baseFromParticiple(s.Words[j].Lower())
	} else {
		// "has been read" and "must not be read" carry a word no rewrite here moves.
		if before := s.Words[i-1]; before.Tag == "MD" || haveForms.Contains(before.Lower()) || beForms.Contains(before.Lower()) {
			return "", false
		}
		verb = passiveVerb(s.Words[j].Lower(), s.Words[i].Lower(), !means && s.Plural(syntax.Phrase{Head: actorHead(s, by+1, end)}))
	}
	// A finite verb in the subject outside a relative clause makes the words in
	// front of the passive a clause of their own.
	if verbOutsideRelative(s, first, subjectEnd) {
		return "", false
	}
	from, to := s.Words[first].Start, s.Words[end].End
	subject, subjectMark := markOf(source, from, s.Words[subjectEnd].Start)
	// A comma that closed the subject before the verb goes with the clause it ended, not with the subject the rewrite moves behind the verb.
	subject = strings.TrimSpace(strings.TrimRight(subject, ","))
	actor, actorMark := markOf(source, s.Words[by+1].Start, to)
	// A focusing adverb in front of "by" stays with the actor, and "by" goes.
	if by > j+1 && focusing.Contains(s.Words[by-1].Lower()) {
		actor = source[s.Words[by-1].Start:s.Words[by-1].End] + " " + actor
	}
	_, verbMark := markOf(source, s.Words[j].Start, s.Words[j].End)
	if subject == "" || actor == "" || verb == "" {
		return "", false
	}
	object := subject
	if first == 0 {
		actor = capitalizeOpening(actor)
		object = lowerSubject(subject, s.Words[0].Tag)
	}
	// A run that opens right before the clause.
	if subjectMark != "" && strings.HasSuffix(source[:from], subjectMark) {
		from -= len(subjectMark)
	}
	if actorMark != "" && strings.HasPrefix(source[to:], actorMark) {
		to += len(actorMark)
	}
	clause := actorMark + actor + actorMark + " " + verbMark + verb + verbMark + " " + subjectMark + object + subjectMark
	return source[:from] + clause + source[to:], true
}

// lowerSubject writes a subject that opened the sentence as it reads behind
// the verb.
func lowerSubject(subject, tag string) string {
	if (tag == "NNP" || tag == "NNPS") && !NameLike(subject) {
		return lowerOpening(subject)
	}
	return lowerOpeningFor(subject, tag)
}

// byBelongsToOther reports a "by" that another participle governs, as in "is
// recorded in a file keyed by `session_id`". That phrase names the actor of
// "keyed", not of "recorded", so the clause has no actor of its own to move.
func byBelongsToOther(s *syntax.Sentence, participle, by int) bool {
	for k := by - 1; k > participle; k-- {
		w := s.Words[k]
		if w.Tag == "RB" || w.Lower() == "not" {
			continue
		}
		return w.Tag == "VBN" || w.Tag == "VBG" || w.Tag == "VBD"
	}
	return false
}

// verbOutsideRelative reports a finite verb after the first word of the subject
// that no relative pronoun before it governs.
func verbOutsideRelative(s *syntax.Sentence, first, end int) bool {
	relative := false
	for k := first; k < end; k++ {
		w := s.Words[k]
		if w.Tag == "WDT" || w.Tag == "WP" || w.Lower() == "that" {
			relative = true
		}
		if k > first && !relative && (w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "VBD" || w.Tag == "MD") {
			return true
		}
	}
	return false
}

// actorHead answers the index of the noun the actor names: the head of the noun
// phrase that opens it, or the last noun in it. "the rules above for that
// worktree" names the rules, so the verb agrees with them.
func actorHead(s *syntax.Sentence, first, end int) int {
	if ph, ok := s.PhraseAt(first); ok && ph.Kind == syntax.NounPhrase && ph.First <= first && ph.Head <= end {
		return ph.Head
	}
	for k := end; k >= first; k-- {
		if strings.HasPrefix(s.Words[k].Tag, "NN") {
			return k
		}
	}
	return first
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
		if s.Words[k].Text == ":" || s.Words[k].Text == "," && leadsClause(s, first, k) && !joinsSubject(s, first, k) {
			first = k + 1
		}
	}
	for first < i && (s.Words[first].Tag == "CC" || syntax.Is(s.Words[first].Lower(), "subordinator") || s.Words[first].Lower() == "so") {
		first++
	}
	// A subject opens on a noun phrase.
	if first >= i || strings.HasPrefix(s.Words[first].Tag, "W") {
		return 0, false
	}
	if strings.HasPrefix(s.Words[first].Tag, "VB") && !participleModifiesNoun(s, first) {
		return 0, false
	}
	return first, true
}

// joinsSubject reports a comma at k that a coordinator follows, after words
// that hold no verb and open on no adverbial. The words on both sides of the
// coordinator are one coordinated subject, as in "Work in a tree, and the files
// of a clean tree, are judged by the rules".
func joinsSubject(s *syntax.Sentence, first, k int) bool {
	if k+1 >= len(s.Words) || s.Words[k+1].Tag != "CC" || first >= k {
		return false
	}
	switch s.Words[first].Tag {
	case "IN", "RB", "TO", "VBG":
		return false
	}
	for n := first; n < k; n++ {
		switch s.Words[n].Tag {
		case "VBZ", "VBP", "VBD", "MD":
			if !participleModifiesNoun(s, n) {
				return false
			}
		}
	}
	return true
}

// participleModifiesNoun reports a participle that describes the noun after
// it, as in "Unpushed commits". Any other participle opens a reduced clause.
func participleModifiesNoun(s *syntax.Sentence, i int) bool {
	switch s.Words[i].Tag {
	case "VBN", "VBG", "VBD":
	default:
		return false
	}
	return i+1 < len(s.Words) && strings.HasPrefix(s.Words[i+1].Tag, "NN")
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

// markOf answers the text of the span from..to without the emphatic markers
// that mark it, and those markers. A rewrite puts them back around the words
// it moved. A marker sits outside the span or inside it. This is because the
// parser answers a word's offsets and a marker's own, never both.
// `**started**`, `**the gate**` and `__the gate__` each answer their words
// and their own pair.
func markOf(source string, from, to int) (string, string) {
	text := strings.TrimSpace(source[from:to])
	for _, mark := range []string{"**", "__"} {
		opens := strings.HasSuffix(source[:from], mark) || strings.HasPrefix(text, mark)
		closes := strings.HasPrefix(source[to:], mark) || strings.HasSuffix(text, mark)
		if opens && closes {
			return strings.TrimSpace(strings.Trim(text, mark)), mark
		}
	}
	return text, ""
}
