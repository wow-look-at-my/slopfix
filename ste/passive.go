package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// rewritePassive writes the active voice for a passive clause that names its
// actor: "The file is read by the gate" becomes "The gate reads the file".
// A clause that names no actor is left as written, because nothing states who
// acts.
func rewritePassive(s *syntax.Sentence, source string) (string, bool) {
	last := lastContent(s)
	i, j := auxiliary(s, 1, beForms)
	if i < 1 || j < 0 || s.Words[j].Tag != "VBN" {
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
	from, to := s.Words[first].Start, s.Words[end].End
	subject := strings.TrimSpace(unbold(source[from:s.Words[i].Start]))
	actor := strings.TrimSpace(unbold(source[s.Words[by+1].Start:to]))
	verb := passiveVerb(s.Words[j].Lower(), s.Words[i].Lower(), s.Plural(syntax.Phrase{Head: end}))
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

// lowerOpeningFor writes a phrase as it reads inside a sentence rather than at
// the start of one. A name keeps its capital.
func lowerOpeningFor(phrase, tag string) string {
	if tag == "NNP" || tag == "NNPS" {
		return phrase
	}
	first, width := utf8.DecodeRuneInString(phrase)
	if !unicode.IsUpper(first) {
		return phrase
	}
	return string(unicode.ToLower(first)) + phrase[width:]
}

// passiveVerb answers the active form of a passive verb group: the participle
// for a past clause, and the present tense of the base form otherwise.
func passiveVerb(participle, beWord string, plural bool) string {
	switch beWord {
	case "was", "were", "been", "being":
		return pastFromParticiple(participle)
	case "be":
		return pastFromParticiple(participle)
	}
	return presentOf(baseFromParticiple(participle), plural)
}

// baseFromParticiple answers the verb a participle belongs to. A regular verb
// spells its participle with -ed, so the ending answers it by rule. The table
// answers each verb the ending hides.
func baseFromParticiple(form string) string {
	form = strings.ToLower(form)
	if v, ok := verbBy(form); ok && v.participle == form {
		return v.base
	}
	if !strings.HasSuffix(form, "ed") || len(form) <= 3 {
		return form
	}
	stem := form[:len(form)-2]
	if doubled(stem) {
		return stem[:len(stem)-1]
	}
	if strings.HasSuffix(stem, "i") {
		return stem[:len(stem)-1] + "y"
	}
	if v, ok := verbBy(stem + "e"); ok && v.base == stem+"e" {
		return stem + "e"
	}
	if silentE(stem) {
		return stem + "e"
	}
	return stem
}

// silentE reports whether English spelling drops an e from the end of stem before
// -ed. Endings like -dge, -ate, -ize, -ve, -ce and -ure need the e. A stem of one
// syllable that ends on a single vowel and a single consonant took the e as well.
// Because a stem without it doubles its consonant: "named" against "planned".
func silentE(stem string) bool {
	n := len(stem)
	if n < 2 {
		return false
	}
	last, prev := stem[n-1], stem[n-2]
	consonant := func(c byte) bool { return c >= 'a' && c <= 'z' && !strings.ContainsRune("aeiou", rune(c)) }
	switch {
	case last == 'v', last == 'u' && prev != 'q':
		return true
	case last == 'c', last == 'z' && strings.ContainsRune("iy", rune(prev)):
		return true
	case last == 'g' && prev == 'd':
		return true
	case last == 's' && strings.ContainsRune("rnpl", rune(prev)):
		return true
	case last == 'l' && consonant(prev) && !strings.ContainsRune("lrw", rune(prev)):
		return true
	case n >= 3 && strings.ContainsRune("aiu", rune(prev)) && strings.ContainsRune("tr", rune(last)) && consonant(stem[n-3]):
		// -ate, -ire, -ure and -are, after a consonant.
		return prev != 'i' || last == 'r'
	case n >= 3 && prev == 'i' && strings.ContainsRune("nd", rune(last)) && consonant(stem[n-3]):
		// -ine and -ide, after a consonant.
		return true
	}
	// The u of qu spells a consonant sound.
	w := strings.ReplaceAll(stem, "qu", "q")
	n = len(w)
	return n >= 2 && oneSyllable(w) && strings.ContainsRune("aeiou", rune(w[n-2])) && (n == 2 || consonant(w[n-3])) &&
		consonant(last) && !strings.ContainsRune("wxy", rune(last))
}

// oneSyllable reports whether word holds a single run of vowels. A y opens no
// run at the start of a word, where it spells a consonant.
func oneSyllable(word string) bool {
	runs := 0
	vowel := false
	for i := 0; i < len(word); i++ {
		v := strings.ContainsRune("aeiou", rune(word[i])) || word[i] == 'y' && i > 0
		if v && !vowel {
			runs++
		}
		vowel = v
	}
	return runs == 1
}
