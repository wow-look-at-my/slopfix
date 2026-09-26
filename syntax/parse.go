package syntax

import (
	"strings"
	"sync"

	"github.com/jdkato/prose/v3/tag"
	"github.com/jdkato/prose/v3/tokenize"
)

// tagger loads the model a single time per process, because decoding it is the costly part.
var tagger = sync.OnceValue(func() *tag.Tagger {
	t, err := tag.New()
	if err != nil {
		panic("syntax: the tagger model does not load: " + err.Error())
	}
	return t
})

var tokenizer = sync.OnceValue(func() *tokenize.Tokenizer { return tokenize.New() })

// Parse tags and parses a sentence. A word inside an opaque span, such as a
// code span, is data rather than English, so it is read as a name.
func Parse(text string, opaque [][]int) *Sentence {
	tokens := tokenizer().Tokenize(text)
	tagger().TagTokens(tokens)
	s := &Sentence{Text: text}
	for _, t := range tokens {
		w := Word{Text: t.Text, Tag: t.Tag, Start: t.Start, End: t.End()}
		if inside(opaque, w.Start, w.End) {
			w.Tag = "NNP"
		}
		s.Words = append(s.Words, w)
	}
	retag(s.Words)
	s.Phrases = chunk(s.Words)
	if restoreVerbs(s.Words, s.Phrases) {
		s.Phrases = chunk(s.Words)
	}
	s.Clauses = clauses(s)
	return s
}

// restoreVerbs finds a stretch between clause boundaries that has no finite
// verb, and rereads the last noun of a compound in it as the verb. The tagger
// reads "the write fails" as a plural compound noun, because "fails" follows
// "write" the way "names" follows "file".
func restoreVerbs(words []Word, phrases []Phrase) bool {
	changed := false
	start := 0
	for i := 0; i <= len(words); i++ {
		if i < len(words) && !divides(words[i]) {
			continue
		}
		if !hasFiniteVerb(words[start:i]) {
			changed = restoreOne(words, phrases, start, i) || changed
		}
		start = i + 1
	}
	for n := 1; n < len(phrases); n++ {
		// A compound right after another noun phrase opens a clause of its own: "the reason a caller waits".
		if prev := phrases[n-1]; prev.Kind == NounPhrase && prev.Last+1 == phrases[n].First {
			changed = restoreOne(words, phrases[n:n+1], phrases[n].First, phrases[n].Last+1) || changed
		}
	}
	return changed
}

// divides reports a word that separates clauses: a comma, a conjunction, a
// subordinator or a relative word.
func divides(w Word) bool {
	switch w.Tag {
	case ",", ":", "CC", "WDT", "WP":
		return true
	case "IN", "WRB":
		return Is(w.Text, "subordinator")
	}
	return false
}

func hasFiniteVerb(words []Word) bool {
	for n, w := range words {
		if isFinite(w.Tag) || n == 0 && w.Tag == "VB" {
			return true
		}
	}
	return false
}

// restoreOne retags the head of the earliest compound in [from, to) whose
// number shows it is a verb. A singular noun followed by an -s form is a
// subject and its verb. A plural noun followed by a bare form is too.
func restoreOne(words []Word, phrases []Phrase, from, to int) bool {
	for _, p := range phrases {
		if p.Kind != NounPhrase || p.First < from || p.Last >= to || p.Head <= p.First {
			continue
		}
		head, before := &words[p.Head], words[p.Head-1]
		switch {
		case head.Tag == "NNS" && (before.Tag == "NN" || before.Tag == "NNP"):
			head.Tag = "VBZ"
		case head.Tag == "NN" && before.Tag == "NNS":
			head.Tag = "VBP"
		case head.Tag == "NN" && before.Tag == "NN" && verbEnding(head.Text):
			head.Tag = "VBZ"
		default:
			continue
		}
		return true
	}
	return false
}

func inside(spans [][]int, start, end int) bool {
	for _, span := range spans {
		if start < span[1] && span[0] < end {
			return true
		}
	}
	return false
}

// retag corrects the tags a clause test depends on and the tagger gets wrong.
func retag(words []Word) {
	for i := range words {
		w := &words[i]
		lower := w.Lower()
		switch {
		case lower == "n't" || lower == "not":
			w.Tag = "RB"
		case strings.Trim(w.Text, "x") == "" && w.Tag != "NNP" && len(w.Text) > 3:
			// A masked span reads as a run of x, and names the data it hides.
			w.Tag = "NNP"
		case Is(lower, "coordinator") && w.Tag != "CC" && lower != "so" && lower != "yet":
			w.Tag = "CC"
		case w.Tag == "NNS" && i > 0 && i+1 < len(words) && words[i-1].Tag == "CC" && opensNounPhrase(words[i+1]):
			// "and reads every row": a plural noun cannot take a determiner after it.
			w.Tag = "VBZ"
		case w.Tag == "NNS" && i > 0 && i+1 < len(words) && words[i-1].Tag == "NN" && Is(words[i+1].Text, "object"):
			// "a message reads it": a noun takes no object pronoun.
			w.Tag = "VBZ"
		}
	}
}

// verbEnding reports an -s ending a verb takes, and not "status" or "access".
func verbEnding(word string) bool {
	lower := strings.ToLower(word)
	return strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") &&
		!strings.HasSuffix(lower, "us") && !strings.HasSuffix(lower, "is")
}

func opensNounPhrase(w Word) bool {
	return w.Tag == "DT" || w.Tag == "PRP$" || w.Tag == "CD" || w.Tag == "PDT" || Is(w.Text, "indefinite")
}

func isNoun(tag string) bool {
	return tag == "NN" || tag == "NNS" || tag == "NNP" || tag == "NNPS"
}

func isAdjective(tag string) bool { return tag == "JJ" || tag == "JJR" || tag == "JJS" }

func isVerb(tag string) bool { return strings.HasPrefix(tag, "VB") || tag == "MD" }

func isFinite(tag string) bool {
	return tag == "MD" || tag == "VBZ" || tag == "VBP" || tag == "VBD"
}

func isDeterminer(w Word) bool {
	return w.Tag == "DT" || w.Tag == "PRP$" || w.Tag == "WP$"
}

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
	for j < len(words) {
		tag := words[j].Tag
		if isVerb(tag) || tag == "TO" || tag == "RP" {
			p.Head = j
			j++
			continue
		}
		if (tag == "RB" || tag == "RBR") && j+1 < len(words) && isVerb(words[j+1].Tag) {
			j++
			continue
		}
		break
	}
	p.Last = p.Head
	return p, true
}
