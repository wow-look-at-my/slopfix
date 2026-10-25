// clauseParser walks a chunked sentence and cuts it into clauses.

package syntax

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
				if kind == Relative && cur.Kind == Relative && link > 0 && words[link-1].Tag == "CC" {
					// "that is absent, or that appears": both clauses describe the same noun, at the same depth.
					depth = cur.Depth
				}
				if kind == Relative && cur.Kind == Coordinate && len(out) >= 2 && out[len(out)-2].Kind == Relative {
					// "whose code is present but whose behavior is wrong": the second relative describes the same noun as the first.
					depth = out[len(out)-2].Depth
				}
				cur = Clause{First: i, Link: link, Kind: kind, Comma: comma, Depth: depth}
			}
		}
		comma = false
		if !opens {
			if next, ok := p.resume(i, cur, out); ok {
				cur.Last = p.lastWord(cur.First, next.First-1)
				out = append(out, cur)
				cur = next
				if cur.Depth == 0 {
					p.main = cur
				}
				continue
			}
		}
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
	case w.Text == ";" || w.Text == ":" || w.Text == "—" || w.Text == "–" || w.Text == "--":
		return Punctuated, i, true
	case w.Tag == "CC" && cur.Link >= 0 && cur.Verb == nil && !comma && p.subjectVerbAt(cur.Link+1) && p.isNounPhraseAt(cur.Link+1):
		// "so a signing key or a hook cannot change": the conjunction joins the halves of the clause's subject.
	case w.Tag == "CC" || lower == "so" && comma:
		if p.opensAfterConjunction(i+1, cur) {
			return Coordinate, i, true
		}
	case lower == "that" && i > 0 && p.s.Words[i-1].Tag == "CC" && cur.Kind == Relative:
		// "that is absent, or that appears": the second relative clause describes the same noun.
		return Relative, i, true
	case p.subordinator(i):
		return Subordinate, i, true
	case p.relative(i):
		return Relative, i, true
	case comma && cur.Depth > 0 && (cur.Verb != nil || p.opensSentence(cur)) && p.subjectVerbAt(i):
		// "When using the flag, you need": an opening clause with no finite verb ends at its comma too.
		return Opens, -1, true
	case comma && cur.Kind == Subordinate && cur.Depth == 1 && p.opensSentence(cur) && p.imperativeAt(i):
		// "If the cache is cold, run the build": the instruction after the comma is the main clause.
		return Opens, -1, true
	}
	return 0, 0, false
}

// opensSentence reports a clause that the sentence opens on, after at most a
// label and its colon: "CRITICAL: If the cache is cold".
func (p *clauseParser) opensSentence(c Clause) bool {
	for i := 0; i < c.First; i++ {
		if t := p.s.Words[i].Tag; !punctuation(t) && !(i == 0 && len(p.s.Words) > 1 && p.s.Words[1].Text == ":") {
			return false
		}
	}
	return c.Kind == Subordinate || c.First == 0
}

// imperativeAt reports a bare verb group at word i, after any adverb.
func (p *clauseParser) imperativeAt(i int) bool {
	vg, ok := p.phrase(i)
	return ok && vg.Kind == VerbGroup && vg.First == i && p.s.Words[p.lead(vg)].Tag == "VB"
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
	if _, gerund := p.gerundSubject(j); gerund {
		return true
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
	if _, ok := p.gerundSubject(j); ok {
		return true
	}
	ph, ok := p.phrase(j)
	if !ok || ph.Kind != NounPhrase || ph.First != j {
		return false
	}
	k := p.chainEnd(ph) + 1
	// "a signing key or a hook on the machine cannot", "a hook, a CI job and an editor integration all get": a coordinated subject.
	for k+1 < len(p.s.Words) && (p.s.Words[k].Tag == "CC" || p.s.Words[k].Text == ",") && p.isNounPhraseAt(k+1) {
		next, _ := p.phrase(k + 1)
		k = p.chainEnd(next) + 1
	}
	if k < len(p.s.Words) && (quantifiers.Contains(p.s.Words[k].Lower()) || postposed.Contains(p.s.Words[k].Lower())) {
		k++
	}
	k = p.skipModifier(k)
	for k < len(p.s.Words) && p.s.Words[k].Tag == "RB" {
		if vg, ok := p.phrase(k); ok && vg.Kind == VerbGroup {
			break
		}
		k++
	}
	vg, ok := p.phrase(k)
	return ok && vg.Kind == VerbGroup && vg.Finite
}

// skipModifier answers the word after a clause that describes the noun before
// k, or k. One kind sits between commas: ", which sends stderr to the
// terminal,". The other has no relative word: "the message it writes".
func (p *clauseParser) skipModifier(k int) int {
	words := p.s.Words
	if k+1 < len(words) && words[k].Text == "," && p.relative(k+1) {
		for j := k + 2; j < len(words); j++ {
			switch {
			case words[j].Text == ",":
				return j + 1
			case words[j].Tag == "." || words[j].Tag == ":" || words[j].Tag == "CC":
				return k
			}
		}
		return k
	}
	if end, ok := p.reducedRelative(k); ok {
		return end
	}
	return k
}

// reducedRelative reports a subject and a finite verb at k that a later
// finite verb follows, with no mark between: "it writes to comply puts". It
// answers where that later verb starts.
func (p *clauseParser) reducedRelative(k int) (int, bool) {
	words := p.s.Words
	if k >= len(words) || words[k].Tag != "PRP" && !p.isNounPhraseAt(k) {
		return k, false
	}
	j := k
	if ph, ok := p.phrase(k); ok && ph.Kind == NounPhrase {
		j = ph.Last + 1
	} else {
		j++
	}
	inner, ok := p.phrase(j)
	if !ok || inner.Kind != VerbGroup || inner.First != j || !inner.Finite {
		return k, false
	}
	for i := inner.Last + 1; i < len(words); i++ {
		t := words[i].Tag
		if punctuation(t) || t == "CC" || t == "WDT" || t == "WP" || p.subordinator(i) {
			return k, false
		}
		if vg, ok := p.phrase(i); ok && vg.Kind == VerbGroup && vg.First == i && vg.Finite {
			return i, true
		}
	}
	return k, false
}

// gerundSubject answers a gerund and its object at j that a finite verb
// follows, as the subject of that verb: "adding a rule is".
func (p *clauseParser) gerundSubject(j int) (Phrase, bool) {
	words := p.s.Words
	if j >= len(words) || words[j].Tag != "VBG" {
		return Phrase{}, false
	}
	k := j + 1
	if ph, ok := p.phrase(k); ok && ph.Kind == NounPhrase && ph.First == k {
		k = p.chainEnd(ph) + 1
	}
	vg, ok := p.phrase(k)
	if !ok || vg.Kind != VerbGroup || vg.First != k || !vg.Finite {
		return Phrase{}, false
	}
	return Phrase{Kind: NounPhrase, First: j, Last: k - 1, Head: j, Det: -1}, true
}
