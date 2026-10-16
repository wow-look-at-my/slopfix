package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// rewritePassive writes a passive clause that carries a "by" phrase after its
// participle in a form with no passive. A "by" phrase that names an actor
// becomes the subject: "The file is read by the gate" becomes "The gate reads
// the file". A "by" phrase that names a means or a manner names no actor. A
// clause that opens the sentence then becomes an instruction: "Repos are
// reached by making a token" becomes "Reach repos by making a token".
// Elsewhere, and for a participle that is no verb, the "by" phrase moves in
// front of the subject: "since the checks are red by definition" becomes
// "since by definition the checks are red".
func rewritePassive(s *syntax.Sentence, source string) (string, bool) {
	last := lastContent(s)
	i, j := auxiliary(s, 1, beForms)
	if i < 1 || j < 0 || s.Words[j].Tag != "VBN" || !simpleSpan(s, i, j) {
		return "", false
	}
	by := -1
	for k := j + 1; k < last; k++ {
		if s.Words[k].Lower() == "by" {
			by = k
			break
		}
	}
	if by < 0 {
		return "", false
	}
	end := last
	for k := by + 1; k <= last; k++ {
		if s.Words[k].Text == "," {
			end = k - 1
			break
		}
	}
	if end < by+1 {
		return "", false
	}
	start := clauseOpening(s, i)
	if strings.TrimSpace(source[s.Words[start].Start:s.Words[i].Start]) == "" {
		return "", false
	}
	p := passiveClause{s: s, source: source, start: start, be: i, participle: j, by: by, end: end}
	verb := participleOfVerb(s.Words[j].Lower())
	switch {
	case verb && p.namesActor():
		return p.active(), true
	case verb && p.opensInstruction():
		return p.instruction(), true
	}
	return p.frontedBy(), true
}

// passiveClause holds the word indexes of a passive clause: where its subject
// starts, its form of be, its participle.
type passiveClause struct {
	s                              *syntax.Sentence
	source                         string
	start, be, participle, by, end int
}

// clauseOpening answers the first word of the clause that holds word i. A
// colon, a semicolon, a dash, a subordinator and a comma before a conjunction
// open a clause. A comma alone does not, because a subject can list nouns.
func clauseOpening(s *syntax.Sentence, i int) int {
	for k := i - 1; k > 0; k-- {
		w := s.Words[k]
		switch {
		case w.Text == ":" || w.Text == ";" || dashMark(w.Text):
			return openingAt(k+1, i)
		case (w.Tag == "IN" || w.Tag == "WRB") && syntax.Is(w.Text, "subordinator"):
			return openingAt(k+1, i)
		case w.Tag == "CC" && s.Words[k-1].Text == ",":
			return openingAt(k+1, i)
		}
	}
<<<<<<< HEAD
	return 0
=======
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
>>>>>>> origin/master
}

// openingAt answers k when words stand between k and the verb at i, and the
// start of the sentence otherwise.
func openingAt(k, i int) int {
	if k >= i {
		return 0
	}
	return k
}

<<<<<<< HEAD
// participleOfVerb reports a participle a verb writes: a form the verb table
// lists, or a regular one in -ed. "red" ends in -ed and is an adjective.
func participleOfVerb(form string) bool {
=======
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
	}
	return presentOf(baseFromParticiple(participle), plural)
}

// baseFromParticiple answers the verb a participle belongs to. A regular verb
// spells its participle with -ed, so the ending answers it by rule. The table
// answers each verb the ending hides.
func baseFromParticiple(form string) string {
	form = strings.ToLower(form)
>>>>>>> origin/master
	if v, ok := verbBy(form); ok && v.participle == form {
		return true
	}
	return strings.HasSuffix(form, "ed") && len(form) > 3
}

// namesActor reports a "by" phrase that names who acts. A phrase that opens on
// a determiner, a pronoun, a number or a name, or one whose head is plural. A
// bare singular noun names a manner, as in "by definition" and "by full URL".
// A gerund names a means.
func (p passiveClause) namesActor() bool {
	w := p.s.Words[p.by+1]
	switch w.Tag {
	case "DT", "PRP", "PRP$", "CD", "NNP", "NNPS", "NNS":
		return true
	}
	if syntax.Is(w.Text, "definite") || syntax.Is(w.Text, "indefinite") {
		return true
	}
	ph, ok := p.s.PhraseAt(p.by + 1)
	if !ok || ph.Kind != syntax.NounPhrase {
		return false
	}
	head := p.s.Words[ph.Head].Tag
	return head == "NNS" || head == "NNPS"
}

// opensInstruction reports a clause that opens the sentence, or opens it. This
// happens after a label and a colon, so that a bare verb in its place reads as
// an instruction.
func (p passiveClause) opensInstruction() bool {
	if p.start == 0 {
		return true
	}
	colon := p.start - 1
	if p.s.Words[colon].Text != ":" {
		return false
	}
	for k := 0; k < colon; k++ {
		if finiteVerbTag(p.s.Words[k].Tag) || p.s.Words[k].Tag == "MD" {
			return false
		}
	}
	return true
}

// subject answers the words from the clause's opening to its form of be.
func (p passiveClause) subject() string {
	return strings.TrimSpace(p.source[p.s.Words[p.start].Start:p.s.Words[p.be].Start])
}

// pronoun answers the object pronoun that stands for the subject after a later verb.
func (p passiveClause) pronoun() string {
	if ph, ok := p.s.PhraseAt(p.start); ok && ph.Kind == syntax.NounPhrase && p.s.Plural(*ph) {
		return "them"
	}
	return "it"
}

// coordinated answers the words between the participle and "by", with each
// later participle that a conjunction joins to it written by form. It given
// the subject as its object. "and named" becomes "and name it".
func (p passiveClause) coordinated(form func(string) string) string {
	var out strings.Builder
	at := p.s.Words[p.participle].End
	for k := p.participle + 1; k < p.by; k++ {
		w := p.s.Words[k]
		if (w.Tag == "VBN" || w.Tag == "VBD") && p.s.Words[k-1].Tag == "CC" {
			out.WriteString(p.source[at:w.Start])
			out.WriteString(form(w.Lower()) + " " + p.pronoun())
			at = w.End
		}
	}
	out.WriteString(p.source[at:p.s.Words[p.by].Start])
	return out.String()
}

// active names the actor as the subject, and the subject as the object.
func (p passiveClause) active() string {
	w := p.s.Words
	plural := p.s.Plural(syntax.Phrase{Head: p.end})
	be := w[p.be].Lower()
	form := func(participle string) string { return passiveVerb(participle, be, plural) }
	actor := p.source[w[p.by+1].Start:w[p.end].End]
	object := p.subject()
	if p.start == 0 {
		actor = capitalizeOpening(actor)
		object = lowerOpeningFor(object, w[p.start].Tag)
	}
	rest := strings.TrimRight(p.coordinated(form), " ,")
	return p.source[:w[p.start].Start] + actor + " " + form(w[p.participle].Lower()) + " " + object + rest + p.source[w[p.end].End:]
}

// instruction writes the clause as an instruction to the reader, who is the
// actor a clause with no actor leaves.
func (p passiveClause) instruction() string {
	w := p.s.Words
	verb := baseFromParticiple(w[p.participle].Lower())
	object := p.subject()
	if p.start == 0 || isCapitalized(object) {
		verb = capitalizeOpening(verb)
	}
	if p.start == 0 {
		object = lowerOpeningFor(object, w[p.start].Tag)
	}
	return p.source[:w[p.start].Start] + verb + " " + object + p.coordinated(baseFromParticiple) + p.source[w[p.by].Start:]
}

// frontedBy moves the "by" phrase in front of the subject, which leaves the
// clause with no passive "by" phrase.
func (p passiveClause) frontedBy() string {
	w := p.s.Words
	phrase := p.source[w[p.by].Start:w[p.end].End]
	clause := strings.TrimRight(p.source[w[p.start].Start:w[p.by].Start], " ,")
	tail := p.source[w[p.end].End:]
	if p.start == 0 {
		return capitalizeOpening(phrase) + ", " + lowerOpeningFor(clause, w[p.start].Tag) + tail
	}
	return p.source[:w[p.start].Start] + phrase + " " + clause + tail
}
