// The actor is the phrase after "by", and it must close the clause.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// rewritePassive writes a passive clause that carries a "by" phrase after its
// participle in a form with no passive.
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
	if participleOfVerb(s.Words[j].Lower()) && p.namesActor() {
		return "", false
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
