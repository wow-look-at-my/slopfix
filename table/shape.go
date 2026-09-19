// shape.go matches a sentence STRUCTURE rather than a span of bytes.
//
// A cardinal is a determiner when it counts the noun after it, and a pronoun
// when it stands for a noun instead. Which of those a word is cannot be read
// off the word. It is the slots on each side that say. A regexp cannot ask
// what fills the slot behind a match, so the shape that says "not where an
// article already governs this" has to grow into the match itself, and then
// enumerate an open class of adjectives and verbs it can never finish.
//
// So rules/ declares the structure with a tag per slot, and this walks the
// prose as words instead of bytes.
package table

import "strings"

// A Class is a set of words that fill the same slot. rules/ declares each one,
// and a class holds only a CLOSED set: articles, determiners, prepositions,
// relative pronouns, conjunctions, auxiliaries, adverbs. An open class such as
// a noun or a verb is what is left after every closed class has answered.
type Class struct {
	Name  string
	Words []string
	// Suffix claims a word by its ending rather than by a list. An open class
	// has no list to write, and a verb form is what its ending says it is.
	Suffix []string
}

// A Slot is a tag inside a shape. It matches by class, or by a literal word
// where the rule names the word it is about.
type Slot struct {
	// Classes are the classes the word may belong to, and any of them fits.
	// "open" matches a word no class with a word list claims, which is a noun
	// or a verb.
	Classes []string
	// Word is a literal the token has to equal, ignoring case.
	Word string
	// Capture numbers the group the replacement writes back.
	Capture int
	// Optional lets the slot match nothing.
	Optional bool
	// Absent inverts the slot: it matches where such a word is NOT, and
	// consumes nothing. The slot at the end of a shape reads the word beyond it.
	Absent bool
}

// A Shape is a run of slots and what to say instead. To spells its captures
// $1, $2, the way a pattern does.
type Shape struct {
	Slots  []Slot
	To     string
	Where  string
	Test   string
	Expect string
}

// Lexicon answers which classes a word belongs to.
type Lexicon struct {
	classOf  map[string][]string
	suffixes []Class
}

// NewLexicon indexes the declared classes. A word may belong to several: "one"
// is a cardinal and a pronoun, and which of them it is on the day is what a
// shape is for.
func NewLexicon(classes []Class) *Lexicon {
	lex := &Lexicon{classOf: map[string][]string{}}
	for _, c := range classes {
		for _, w := range c.Words {
			w = strings.ToLower(w)
			lex.classOf[w] = append(lex.classOf[w], c.Name)
		}
		if len(c.Suffix) > 0 {
			lex.suffixes = append(lex.suffixes, c)
		}
	}
	return lex
}

// Is reports whether word belongs to class. The class "open" is every word no
// class with a word list claims, which is a noun or a verb.
func (l *Lexicon) Is(word, class string) bool {
	word = strings.ToLower(word)
	if class == "open" {
		return len(l.classOf[word]) == 0
	}
	for _, got := range l.classOf[word] {
		if got == class {
			return true
		}
	}
	for _, c := range l.suffixes {
		if c.Name != class {
			continue
		}
		for _, s := range c.Suffix {
			if len(word) > len(s) && strings.HasSuffix(word, s) {
				return true
			}
		}
	}
	return false
}

// token is a word and where it sits in the prose.
type token struct {
	at, end int
}

// Apply rewrites every place in prose where the shape matches. Prose the shape
// does not match comes back untouched.
func (s Shape) Apply(lex *Lexicon, prose string) string {
	words := words(prose)
	if len(words) == 0 {
		return prose
	}
	var out strings.Builder
	last := 0
	for i := 0; i < len(words); {
		end, groups, ok := s.matchAt(lex, prose, words, i)
		if !ok {
			i++
			continue
		}
		out.WriteString(prose[last:words[i].at])
		out.WriteString(s.say(prose, groups))
		last = words[end-1].end
		i = end
	}
	out.WriteString(prose[last:])
	return out.String()
}

// matchAt walks the slots against the words from i, and answers the word after
// the match with each captured span.
func (s Shape) matchAt(lex *Lexicon, prose string, words []token, i int) (int, map[int]string, bool) {
	groups := map[int]string{}
	at := i
	for _, slot := range s.Slots {
		if slot.Absent {
			if at < len(words) && slot.fits(lex, text(prose, words[at])) {
				return 0, nil, false
			}
			continue
		}
		if at < len(words) && slot.fits(lex, text(prose, words[at])) {
			if slot.Capture > 0 {
				groups[slot.Capture] = text(prose, words[at])
			}
			at++
			continue
		}
		if slot.Optional {
			continue
		}
		return 0, nil, false
	}
	if at == i {
		return 0, nil, false
	}
	return at, groups, true
}

// fits reports whether a word fills this slot.
func (slot Slot) fits(lex *Lexicon, word string) bool {
	if slot.Word != "" {
		return strings.EqualFold(slot.Word, word)
	}
	for _, class := range slot.Classes {
		if lex.Is(word, class) {
			return true
		}
	}
	return false
}

// say writes the replacement, resolving each capture. A capture that matched
// nothing writes nothing, and the spacing closes around it.
func (s Shape) say(prose string, groups map[int]string) string {
	var out strings.Builder
	for i := 0; i < len(s.To); i++ {
		if s.To[i] != '$' || i+1 >= len(s.To) {
			out.WriteByte(s.To[i])
			continue
		}
		group, width, ok := groupRef(s.To[i+1:])
		if !ok {
			out.WriteByte(s.To[i])
			continue
		}
		i += width
		out.WriteString(groups[group])
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// words indexes the word runs of prose, which is what a slot matches.
func words(prose string) []token {
	var out []token
	for i := 0; i < len(prose); {
		if !isWordByte(prose[i]) {
			i++
			continue
		}
		start := i
		for i < len(prose) && isWordByte(prose[i]) {
			i++
		}
		out = append(out, token{at: start, end: i})
	}
	return out
}

func text(prose string, t token) string { return prose[t.at:t.end] }
