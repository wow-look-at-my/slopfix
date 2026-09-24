// Package syntax parses an English sentence into phrases and clauses.
//
// A perceptron tagger assigns each word a Penn Treebank part of speech.
// Finite-state passes then build the structure: noun phrases, then verb groups,
// then clauses. Each word keeps its byte span in the source, so a rule can
// rewrite the exact text a phrase covers.
package syntax

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// Word is a token of the source and its part of speech.
type Word struct {
	Text string
	// Tag is the Penn Treebank tag. A word inside an opaque span is NNP.
	Tag string
	// Start and End are byte offsets into the text given to Parse.
	Start, End int
}

// Lower is the word in lower case, which is how the word classes list it.
func (w Word) Lower() string { return strings.ToLower(w.Text) }

// Kind is the grammatical type of a phrase.
type Kind int

const (
	// NounPhrase is a noun and the words that modify it.
	NounPhrase Kind = iota
	// VerbGroup is a finite or non-finite verb and its auxiliaries.
	VerbGroup
)

type Phrase struct {
	Kind        Kind
	First, Last int
	// Head is the word the phrase is about.
	Head int
	// Det is the determiner or possessive 's, or negative.
	Det int
	// Numerals are the cardinals between the determiner and the modifiers.
	Numerals []int
	// Finite reports whether a verb group carries tense, which is what makes the clause around it stand alone.
	Finite bool
	// Imperative reports a bare verb that opens its clause, as in "Write the file".
	Imperative bool
}

// LinkKind is how a clause attaches to the clause before it.
type LinkKind int

const (
	// Opens starts the sentence, or resumes the main clause.
	Opens LinkKind = iota
	// Coordinate is a clause after and, but, or, so, yet or nor.
	Coordinate
	// Subordinate is a clause after a subordinator such as because or when.
	Subordinate
	// Relative is a clause after which, who or that.
	Relative
	// Punctuated is a clause after a colon or a semicolon.
	Punctuated
)

// Clause is a subject and a finite verb, and the words that complete them.
type Clause struct {
	First, Last int
	// Link is the word that attaches the clause. It is negative when no word does.
	Link int
	Kind LinkKind
	// Comma reports a comma directly in front of the link.
	Comma bool
	// Subject is nil when the clause shares its subject with an earlier clause, or when a relative word is its subject.
	Subject *Phrase
	// Verb is the clause's finite verb group.
	Verb *Phrase
	// Depth counts the levels of subordination above the clause.
	Depth int
}

// Sentence is a parsed sentence.
type Sentence struct {
	Text    string
	Words   []Word
	Phrases []Phrase
	Clauses []Clause
}

// Span is the source text from word earliest to word last, both included.
func (s *Sentence) Span(first, last int) string {
	if first < 0 || last >= len(s.Words) || first > last {
		return ""
	}
	return s.Text[s.Words[first].Start:s.Words[last].End]
}

// PhraseAt answers the phrase that covers word i.
func (s *Sentence) PhraseAt(i int) (*Phrase, bool) {
	for n := range s.Phrases {
		if p := &s.Phrases[n]; p.First <= i && i <= p.Last {
			return p, true
		}
	}
	return nil, false
}

// NounPhrases answers every noun phrase in source order.
func (s *Sentence) NounPhrases() []Phrase {
	var out []Phrase
	for _, p := range s.Phrases {
		if p.Kind == NounPhrase {
			out = append(out, p)
		}
	}
	return out
}

// Plural reports whether a noun phrase names several things.
func (s *Sentence) Plural(p Phrase) bool {
	switch s.Words[p.Head].Tag {
	case "NNS", "NNPS":
		return true
	case "PRP":
		return set.Of("we", "they", "us", "them").Contains(s.Words[p.Head].Lower())
	}
	return false
}

// Person reports whether a noun phrase names a person, which decides between
// it and they.
func (s *Sentence) Person(p Phrase) bool {
	return classes.Is(s.Words[p.Head].Lower(), "person")
}

// syntaxTable is what rules/ says for="syntax".
var syntaxTable = table.MustLoad(rules.FS, "syntax")

var classes = syntaxTable.Lexicon()

// Is reports whether a word belongs to a named class in rules/syntax.xml.
func Is(word, class string) bool { return classes.Is(strings.ToLower(word), class) }
