// classes.go holds the word classes the grammar's terminals name.
//
// A class here is CLOSED: the language admits no new article and no new
// preposition, so the list is the whole class and stays correct. An open class
// is what is left when no class claims the word, which is a noun or a verb,
// and a production names that with <open/>.
package table

import (
	"slices"
	"strings"
)

// A Class is a set of words that fill the same slot.
type Class struct {
	Name  string
	Words []string
	// Suffix claims a word by its ending rather than by a list.
	Suffix []string
	// Digits claims a word made of ASCII digits alone.
	Digits bool
	// Open claims every word no class with a word list claims.
	Open   bool
	Except []string
}

type Lexicon struct {
	classOf  map[string][]string
	suffixes []Class
	digits   []string
	opens    []Class
}

// NewLexicon indexes the declared classes.
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
		if c.Digits {
			lex.digits = append(lex.digits, c.Name)
		}
		if c.Open {
			lex.opens = append(lex.opens, c)
		}
	}
	return lex
}

// Is reports whether word belongs to class. The class "open" is every word no
// class with a word list claims.
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
	if isDigits(word) && slices.Contains(l.digits, class) {
		return true
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
	for _, c := range l.opens {
		if c.Name != class || len(l.classOf[word]) > 0 {
			continue
		}
		barred := false
		for _, other := range c.Except {
			if l.Is(word, other) {
				barred = true
			}
		}
		if !barred {
			return true
		}
	}
	return false
}

func isDigits(word string) bool {
	if word == "" {
		return false
	}
	for i := 0; i < len(word); i++ {
		if word[i] < '0' || word[i] > '9' {
			return false
		}
	}
	return true
}
