package syntax

import "github.com/wow-look-at-my/go-containers/set"

// clauseParser walks a chunked sentence and cuts it into clauses.
type clauseParser struct {
	s  *Sentence
	at []int // the phrase that covers each word, or a negative index
	// main is the latest main clause with a verb, which a coordinated verb group can return to.
	main Clause
}

func clauses(s *Sentence) []Clause {
	p := clauseParser{s: s, at: make([]int, len(s.Words))}
	for i := range p.at {
		p.at[i] = -1
	}
	for n, ph := range s.Phrases {
		for i := ph.First; i <= ph.Last; i++ {
			p.at[i] = n
		}
	}
	return p.run()
}

func (p *clauseParser) phrase(i int) (*Phrase, bool) {
	if i < 0 || i >= len(p.at) || p.at[i] < 0 {
		return nil, false
	}
	return &p.s.Phrases[p.at[i]], true
}

func (p *clauseParser) run() []Clause {
	words := p.s.Words
	var out []Clause
	cur := Clause{Link: -1, Kind: Opens}
	comma := false
	for i := 0; i < len(words); i++ {
		if words[i].Text == "," {
			comma = true
			continue
		}
		kind, link, opens := p.boundary(i, cur, comma)
		if opens {
			if p.empty(cur, i) {
				// Nothing precedes the link, so the link opens the sentence's own clause.
				cur.Link, cur.Kind, cur.Depth = link, kind, depthOf(kind, Clause{})
			} else {
				cur.Last = p.lastWord(cur.First, i-1)
				out = append(out, cur)
				depth := depthOf(kind, cur)
				if kind == Coordinate && cur.Depth > 0 && p.returnsToMain(i, cur, comma) {
					depth = 0
				}
				cur = Clause{First: i, Link: link, Kind: kind, Comma: comma, Depth: depth}
			}
		}
		comma = false
		if ph, ok := p.phrase(i); ok && ph.Kind == VerbGroup && ph.First == i && cur.Verb == nil {
			p.attach(&cur, ph)
			if cur.Depth == 0 && cur.Verb != nil {
				p.main = cur
			}
		}
	}
	cur.Last = p.lastWord(cur.First, len(words)-1)
	if cur.Last >= cur.First {
		out = append(out, cur)
	}
	return out
}

// depthOf answers the depth of a clause of this kind, after the clause prev.
func depthOf(kind LinkKind, prev Clause) int {
	switch kind {
	case Subordinate, Relative:
		return prev.Depth + 1
	case Coordinate:
		return prev.Depth
	}
	return 0
}

// empty reports whether the clause holds no word before i.
func (p *clauseParser) empty(c Clause, i int) bool {
	return p.lastWord(c.First, i-1) < c.First
}

func (p *clauseParser) lastWord(first, last int) int {
	for last >= first && punctuation(p.s.Words[last].Tag) {
		last--
	}
	return last
}

func punctuation(tag string) bool {
	switch tag {
	case ",", ".", ":", "``", "''", "(", ")", "-LRB-", "-RRB-", "HYPH", "NFP", "$", "#", "SYM":
		return true
	}
	return false
}

// boundary reports whether word i opens a new clause, and how.
func (p *clauseParser) boundary(i int, cur Clause, comma bool) (LinkKind, int, bool) {
	w := p.s.Words[i]
	lower := w.Lower()
	switch {
	case w.Text == ";" || w.Text == ":":
		return Punctuated, i, true
	case w.Tag == "CC" || lower == "so" && comma:
		if p.opensAfterConjunction(i+1, cur) {
			return Coordinate, i, true
		}
	case p.subordinator(i):
		return Subordinate, i, true
	case p.relative(i):
		return Relative, i, true
	case comma && cur.Depth > 0 && cur.Verb != nil && p.subjectVerbAt(i):
		return Opens, -1, true
	}
	return 0, 0, false
}

// opensAfterConjunction reports whether the words from j form a clause, or a
// verb group that shares the subject before the conjunction. Anything else joins
// phrases: "files and directories".
func (p *clauseParser) opensAfterConjunction(j int, cur Clause) bool {
	for j < len(p.s.Words) && p.s.Words[j].Tag == "RB" {
		if ph, ok := p.phrase(j); ok && ph.Kind == VerbGroup {
			break
		}
		j++
	}
	ph, ok := p.phrase(j)
	if !ok {
		return false
	}
	if ph.Kind == VerbGroup {
		if ph.Finite {
			return cur.Verb != nil || p.main.Verb != nil
		}
		return p.main.Verb != nil && p.main.Verb.Imperative && p.s.Words[p.lead(ph)].Tag == "VB"
	}
	return p.subjectVerbAt(j)
}

// returnsToMain reports whether a conjunction at i, inside a subordinate clause,
// joins the main clause instead. A comma before it says so. So does a verb
// that takes the main verb's form rather than the subordinate verb's, as in
// "Write the text that joins them and start".
func (p *clauseParser) returnsToMain(i int, cur Clause, comma bool) bool {
	if comma {
		return true
	}
	if p.main.Verb == nil || cur.Verb == nil {
		return false
	}
	j := i + 1
	for j < len(p.s.Words) && p.s.Words[j].Tag == "RB" {
		j++
	}
	vg, ok := p.phrase(j)
	if !ok || vg.Kind != VerbGroup {
		return false
	}
	form := p.s.Words[p.lead(vg)].Tag
	return form == p.s.Words[p.lead(p.main.Verb)].Tag && form != p.s.Words[p.lead(cur.Verb)].Tag
}

// lead answers the verb that decides a group's tense, after any adverb.
func (p *clauseParser) lead(ph *Phrase) int {
	i := ph.First
	for i < ph.Last && !isVerb(p.s.Words[i].Tag) && p.s.Words[i].Tag != "TO" {
		i++
	}
	return i
}

// subjectVerbAt reports whether a noun phrase, with any prepositional phrases
// on it, opens at j and a finite verb group follows it.
func (p *clauseParser) subjectVerbAt(j int) bool {
	ph, ok := p.phrase(j)
	if !ok || ph.Kind != NounPhrase || ph.First != j {
		return false
	}
	k := p.chainEnd(ph) + 1
	for k < len(p.s.Words) && p.s.Words[k].Tag == "RB" {
		if vg, ok := p.phrase(k); ok && vg.Kind == VerbGroup {
			break
		}
		k++
	}
	vg, ok := p.phrase(k)
	return ok && vg.Kind == VerbGroup && vg.Finite
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

func (p *clauseParser) subordinator(i int) bool {
	w := p.s.Words[i]
	lower := w.Lower()
	if lower == "so" && i+1 < len(p.s.Words) && p.s.Words[i+1].Lower() == "that" {
		return true
	}
	if lower == "that" && w.Tag == "IN" {
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

func (p *clauseParser) relative(i int) bool {
	w := p.s.Words[i]
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
	return ph
}
