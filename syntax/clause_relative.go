// clause_relative.go finds subordinate and relative clauses and attaches each verb group to its clause.

package syntax

import (
	"github.com/wow-look-at-my/go-containers/set"
)

// quantifiers float after a plural subject: "the jobs all run".
var quantifiers = set.Of[string]("all", "both", "each")

// postposed words follow the noun they place: "the case above is".
var postposed = set.Of(WordsOf("postposed")...)

func (p *clauseParser) isNounPhraseEnd(k int) bool {
	ph, ok := p.phrase(k)
	return ok && ph.Kind == NounPhrase && ph.Last == k
}

func (p *clauseParser) isNounPhraseAt(k int) bool {
	ph, ok := p.phrase(k)
	return ok && ph.Kind == NounPhrase && ph.First == k
}

// chainEnd follows a noun phrase through "of the cache" and "in the tree".
func (p *clauseParser) chainEnd(ph *Phrase) int {
	end := ph.Last
	for end+2 < len(p.s.Words) && p.s.Words[end+1].Tag == "IN" && !p.subordinator(end+1) {
		next, ok := p.phrase(end + 2)
		if !ok || next.Kind != NounPhrase || next.First != end+2 {
			break
		}
		end = next.Last
	}
	return end
}

// ambiguous are subordinators that are also prepositions: "after the build".
var ambiguous = set.Of[string]("before", "after", "since", "until", "once")

// asPhrase holds the middle word of the subordinators "as long as" and "as soon as".
var asPhrase = set.Of[string]("long", "soon")

func (p *clauseParser) subordinator(i int) bool {
	w := p.s.Words[i]
	lower := w.Lower()
	if lower == "so" && i+1 < len(p.s.Words) && p.s.Words[i+1].Lower() == "that" {
		return true
	}
	if lower == "that" && w.Tag == "IN" {
		return !p.relativeThat(i)
	}
	if lower == "as" && i+2 < len(p.s.Words) && asPhrase.Contains(p.s.Words[i+1].Lower()) && p.s.Words[i+2].Lower() == "as" {
		return true
	}
	if !Is(lower, "subordinator") {
		return false
	}
	if i+1 < len(p.s.Words) && p.s.Words[i+1].Lower() == "of" {
		return false
	}
	if ambiguous.Contains(lower) {
		return p.subjectVerbAt(i + 1)
	}
	return true
}

// relativeThat reports a "that" the tagger read as a subordinator between a
// noun and a finite verb: "a summary that fits is useful". It opens a relative
// clause. An adverb can stand before the verb: "a clause that never closes".
func (p *clauseParser) relativeThat(i int) bool {
	w := p.s.Words[i]
	if w.Lower() != "that" || i == 0 || !isNoun(p.s.Words[i-1].Tag) {
		return false
	}
	j := i + 1
	for j < len(p.s.Words) && p.s.Words[j].Tag == "RB" {
		j++
	}
	if j < len(p.s.Words) && isFinite(p.s.Words[j].Tag) {
		return true
	}
	return p.objectRelativeAt(j)
}

// objectRelativeAt reports a relative clause with a subject of its own at j,
// whose noun waits for the finite verb after the clause's own: "a run that a
// later commit superseded is still the newest". No comma, conjunction or
// subordinator stands between the clause's verb and the one the noun takes.
func (p *clauseParser) objectRelativeAt(j int) bool {
	if !p.subjectVerbAt(j) {
		return false
	}
	own, ok := p.finiteGroupFrom(j)
	if !ok {
		return false
	}
	next, ok := p.finiteGroupFrom(own.Last + 1)
	if !ok {
		return false
	}
	for k := own.Last + 1; k < next.First; k++ {
		w := p.s.Words[k]
		if w.Text == "," || w.Tag == "CC" || Is(w.Lower(), "subordinator") || Is(w.Lower(), "relative") {
			return false
		}
	}
	return true
}

// finiteGroupFrom answers the first finite verb group that opens at or after word from.
func (p *clauseParser) finiteGroupFrom(from int) (*Phrase, bool) {
	for n := range p.s.Phrases {
		ph := &p.s.Phrases[n]
		if ph.Kind == VerbGroup && ph.Finite && ph.First >= from {
			return ph, true
		}
	}
	return nil, false
}

func (p *clauseParser) relative(i int) bool {
	w := p.s.Words[i]
	if p.relativeThat(i) {
		return true
	}
	switch w.Tag {
	case "WDT", "WP", "WP$":
		return Is(w.Lower(), "relative")
	}
	return false
}

// attach records a verb group as the clause's verb, and finds its subject.
func (p *clauseParser) attach(c *Clause, vg *Phrase) {
	start := c.First
	if c.Link >= 0 {
		start = c.Link + 1
	}
	subject := p.subjectBefore(vg.First-1, start)
	// "so adding a rule is": the gerund and its object are the subject.
	if gerund, ok := p.gerundSubject(start); ok && vg.Finite && gerund.Last+1 == vg.First {
		subject = &gerund
	}
	// "the message it writes to comply puts": the clause's own verb is the later one, and its subject opens the clause.
	if head, ok := p.phrase(start); ok && head.Kind == NounPhrase && head.First == start && vg.Finite {
		end, reduced := p.reducedRelative(p.chainEnd(head) + 1)
		if reduced && end > vg.First {
			return
		}
		if reduced && end == vg.First {
			subject = head
		}
	}
	switch {
	case vg.Finite:
	case subject == nil && p.s.Words[p.lead(vg)].Tag == "VB" && p.onlyAdverbs(start, vg.First):
		vg.Imperative = true
	default:
		return
	}
	c.Verb, c.Subject = vg, subject
}

// onlyAdverbs reports whether nothing but adverbs sits between from and to.
func (p *clauseParser) onlyAdverbs(from, to int) bool {
	for i := from; i < to; i++ {
		if p.s.Words[i].Tag != "RB" {
			return false
		}
	}
	return true
}

// subjectBefore answers the noun phrase that ends at k, extended back through
// any prepositional phrase it carries, or nil.
func (p *clauseParser) subjectBefore(k, start int) *Phrase {
	for k >= start && p.s.Words[k].Tag == "RB" {
		k--
	}
	// "an editor integration all get", "the case above is": a quantifier or a place word after the subject floats off it.
	if k > start && (quantifiers.Contains(p.s.Words[k].Lower()) || postposed.Contains(p.s.Words[k].Lower())) && p.isNounPhraseEnd(k-1) {
		k--
	}
	ph, ok := p.phrase(k)
	if k < start || !ok || ph.Kind != NounPhrase {
		return nil
	}
	for ph.First-2 >= start && p.s.Words[ph.First-1].Tag == "IN" {
		prev, ok := p.phrase(ph.First - 2)
		if !ok || prev.Kind != NounPhrase {
			break
		}
		ph = prev
	}
	if ph.First-2 >= start && p.s.Words[ph.First-1].Tag == "CC" {
		if prev, ok := p.phrase(ph.First - 2); ok && prev.Kind == NounPhrase {
			joined := *prev
			joined.Last, joined.Head, joined.Coordinated = ph.Last, ph.Head, true
			// "a hook, a CI job and an editor integration": the list runs back through its commas.
			for joined.First-2 >= start && p.s.Words[joined.First-1].Text == "," {
				item, ok := p.phrase(joined.First - 2)
				if !ok || item.Kind != NounPhrase {
					break
				}
				joined.First = item.First
			}
			return &joined
		}
	}
	return ph
}

// adjacentNouns reports a noun phrase directly followed by another in [from, to).
func (p *clauseParser) adjacentNouns(from, to int) bool {
	for i := from; i < to; i++ {
		ph, ok := p.phrase(i)
		if !ok || ph.Kind != NounPhrase || ph.First != i {
			continue
		}
		if next, ok := p.phrase(ph.Last + 1); ok && next.Kind == NounPhrase && ph.Last+1 < to {
			return true
		}
	}
	return false
}

// subordinate clause that already has its verb, as the main verb does in "A user
// who reads the message leaves" and "If the cache is cold the build fails".
func (p *clauseParser) resume(i int, cur Clause, out []Clause) (Clause, bool) {
	vg, ok := p.phrase(i)
	if !ok || vg.Kind != VerbGroup || vg.First != i || !vg.Finite || cur.Verb == nil || cur.Depth == 0 {
		return Clause{}, false
	}
	if p.adjacentNouns(cur.Verb.Last+1, i) {
		// "which reads the file the loader holds": the verb belongs to a reduced relative.
		return Clause{}, false
	}
	switch {
	case cur.Kind == Relative && cur.Link > 0:
		for n := len(out) - 1; n >= 0; n-- {
			parent := out[n]
			if parent.Depth != cur.Depth-1 {
				continue
			}
			if parent.Verb != nil {
				// A clause with a verb of its own is not resumed by these words.
				return Clause{}, false
			}
			subject := parent.Subject
			if subject == nil {
				subject = p.subjectBefore(p.lastWord(parent.First, min(cur.Link-1, parent.Last)), parent.First+max(parent.Link+1-parent.First, 0))
			}
			// The relative interrupts a clause still waiting for its verb,
			// and the verb after it resumes that clause.
			if subject == nil {
				return Clause{}, false
			}
			out[n].Verb, out[n].Subject = vg, subject
			return Clause{First: i, Link: -1, Kind: Opens, Depth: parent.Depth, Subject: subject, Verb: vg}, true
		}
	case cur.Kind == Subordinate && cur.Depth == 1 && len(out) == 1 && out[0].First == 0 && out[0].Last == 0 && p.s.Words[0].Tag == "VBG":
		// "Deciding whether X can learn anything needs Y": the gerund and its clause are the subject.
		subject := Phrase{Kind: NounPhrase, First: 0, Last: i - 1, Head: 0, Det: -1}
		return Clause{First: i, Link: -1, Kind: Opens, Subject: &subject, Verb: vg}, true
	case cur.Kind == Subordinate && cur.First == 0 && cur.Depth == 1:
		subject := p.subjectBefore(i-1, cur.Verb.Last+1)
		if subject == nil {
			return Clause{}, false
		}
		return Clause{First: subject.First, Link: -1, Kind: Opens, Subject: subject, Verb: vg}, true
	}
	return Clause{}, false
}
