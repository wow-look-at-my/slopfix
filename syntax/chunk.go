// chunk.go groups tagged words into noun phrases and verb groups.

package syntax

// chunk groups the words into noun phrases and verb groups.
func chunk(words []Word) []Phrase {
	var out []Phrase
	for i := 0; i < len(words); {
		if p, ok := nounPhrase(words, i); ok {
			out = append(out, p)
			i = p.Last + 1
			continue
		}
		if p, ok := verbGroup(words, i); ok {
			out = append(out, p)
			i = p.Last + 1
			continue
		}
		i++
	}
	return out
}

// nounPhrase reads [PDT] [DT] [CD...] [JJ...] N... from word i. A possessive 's
// turns the phrase read so far into the determiner of a longer phrase.
func nounPhrase(words []Word, i int) (Phrase, bool) {
	switch words[i].Tag {
	case "PRP", "EX":
		return Phrase{Kind: NounPhrase, First: i, Last: i, Head: i, Det: -1}, true
	}
	p := Phrase{Kind: NounPhrase, First: i, Head: -1, Det: -1}
	j := i
	if words[j].Tag == "PDT" {
		j++
	}
	for {
		if j < len(words) && isDeterminer(words[j]) {
			p.Det = j
			j++
		}
		p.Numerals = nil
		for j < len(words) && words[j].Tag == "CD" {
			p.Numerals = append(p.Numerals, j)
			j++
		}
		j = modifiers(words, j)
		head := -1
		for j < len(words) && isNoun(words[j].Tag) {
			head = j
			j++
		}
		if head < 0 {
			return pronominal(p, words, j)
		}
		p.Head, p.Last = head, head
		if j < len(words) && words[j].Tag == "POS" {
			p.Det = j
			j++
			continue
		}
		return p, true
	}
}

// modifiers skips the adjectives and participles that come before a noun.
func modifiers(words []Word, j int) int {
	for j < len(words) {
		tag := words[j].Tag
		next := ""
		if j+1 < len(words) {
			next = words[j+1].Tag
		}
		switch {
		case isAdjective(tag), tag == "CD" && (isNoun(next) || isAdjective(next)):
		case (tag == "VBN" || tag == "VBG") && (isNoun(next) || isAdjective(next)) && j > 0 &&
			(isDeterminer(words[j-1]) || isAdjective(words[j-1].Tag)):
		case tag == "HYPH" && j > 0 && isAdjective(words[j-1].Tag):
		default:
			return j
		}
		j++
	}
	return j
}

// pronominal answers a phrase with no noun: "the two", "this", "those three".
func pronominal(p Phrase, words []Word, j int) (Phrase, bool) {
	switch {
	case len(p.Numerals) > 0:
		p.Head = p.Numerals[len(p.Numerals)-1]
		p.Numerals = p.Numerals[:len(p.Numerals)-1]
	case p.Det >= 0 && words[p.Det].Tag == "DT":
		p.Head = p.Det
		p.Det = -1
	default:
		return Phrase{}, false
	}
	p.Last = p.Head
	return p, true
}

// verbGroup reads [MD] [RB...] VB... from word i. A leading adverb joins when a verb follows it.
func verbGroup(words []Word, i int) (Phrase, bool) {
	j := i
	for j < len(words) && (words[j].Tag == "RB" || words[j].Tag == "RBR") {
		j++
	}
	if j >= len(words) || !(isVerb(words[j].Tag) || words[j].Tag == "TO" && j+1 < len(words) && words[j+1].Tag == "VB") {
		return Phrase{}, false
	}
	p := Phrase{Kind: VerbGroup, First: i, Det: -1}
	lead := j
	p.Finite = isFinite(words[lead].Tag)
	// seen is set once the group holds a verb.
	seen := false
	for j < len(words) {
		tag := words[j].Tag
		if isFinite(tag) && seen {
			break
		}
		if isVerb(tag) || tag == "TO" || tag == "RP" {
			seen = seen || isVerb(tag)
			p.Head = j
			j++
			continue
		}
		if (tag == "RB" || tag == "RBR") && j+1 < len(words) && isVerb(words[j+1].Tag) && !(seen && isFinite(words[j+1].Tag)) {
			j++
			continue
		}
		break
	}
	p.Last = p.Head
	return p, true
}
