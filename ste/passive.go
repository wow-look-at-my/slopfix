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
	if out, ok := rewriteActor(s, source); ok {
		return out, true
	}
	return rewriteWithoutActor(s, source)
}

// rewriteWithoutActor writes a passive clause whose "by" phrase names no actor.
func rewriteWithoutActor(s *syntax.Sentence, source string) (string, bool) {
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
	// "has been read", "must not be read" and "must be read" carry a word no rewrite here moves.
	if s.Words[i].Lower() == "be" || s.Words[i-1].Tag == "MD" || haveForms.Contains(s.Words[i-1].Lower()) || beForms.Contains(s.Words[i-1].Lower()) {
		return "", false
	}
	verb := participleOfVerb(s.Words[j].Lower())
	if verb && p.namesActor() {
		return "", false
	}
	switch {
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
	return 0
}

// openingAt answers k when words stand between k and the verb at i, and the
// start of the sentence otherwise.
func openingAt(k, i int) int {
	if k >= i {
		return 0
	}
	return k
}

// participleOfVerb reports a participle a verb writes: a form the verb table
// lists, or a regular one in -ed. "red" ends in -ed and is an adjective.
func participleOfVerb(form string) bool {
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
