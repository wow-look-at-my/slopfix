// Package english is the prose table every rule reads: the words a repair
// drops, the phrasings it swaps, the shapes it rewrites, and the phrases it
// refuses to touch.
//
// The table is XML rather than Go, the same way autoallow carries its rules, so
// adding a rule is a single-line edit somebody can make without reading Go.
package english

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/syntax"
	"github.com/wow-look-at-my/slopfix/table"
)

//go:embed english.xml
var englishXML []byte

// Table mirrors english.xml.
type Table struct {
	Classes  []Class   `xml:"class"`
	Drops    []Drop    `xml:"drop"`
	Rewrites []Rewrite `xml:"rewrite"`
	Patterns []Pattern `xml:"pattern"`
	Shapes   []Shape   `xml:"shape"`
	Flags    []Flag    `xml:"flag"`
	Whole    []Case    `xml:"test"`
}

// Class is a set of words that fill the same slot, stated in a single
// attribute and read as the lexicon a shape asks about each word.
type Class struct {
	Name   string `xml:"name,attr"`
	Words  string `xml:"words,attr"`
	Suffix string `xml:"suffix,attr"`
	Open   bool   `xml:"open,attr"`
	Except string `xml:"except,attr"`
}

// classes indexes the declared word classes, and the indefinite pronouns rules/syntax.xml lists.
var classes = table.NewLexicon(lexicalClasses(loaded.Classes))

func lexicalClasses(declared []Class) []table.Class {
	out := make([]table.Class, 0, len(declared)+1)
	out = append(out, table.Class{Name: "indefinite", Words: syntax.WordsOf("indefinite")})
	for _, c := range declared {
		out = append(out, table.Class{
			Name:   c.Name,
			Words:  strings.Fields(c.Words),
			Suffix: strings.Fields(c.Suffix),
			Open:   c.Open,
			Except: strings.Fields(c.Except),
		})
	}
	return out
}

// Cases is every worked example the table states for itself rather than for a single entry: a line of prose, and what
func Cases() []Case { return loaded.Whole }

// Flag is a phrase the table names but does not rewrite, because no single
// replacement is right. It carries what to write instead.
type Flag struct {
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
	Cases  []Case `xml:"test"`
}

// Flags is every phrase the table names without rewriting.
func Flags() []Flag { return loaded.Flags }

// Tests is every worked example a flag declares.
func (f Flag) Tests() []Case { return cases(f.Test, "", f.Cases) }

// Flagged returns the flags a line carries.
func Flagged(line string) []Flag {
	lower := strings.ToLower(line)
	var out []Flag
	for _, f := range Flags() {
		if ContainsWord(lower, strings.ToLower(f.Phrase)) {
			out = append(out, f)
		}
	}
	return out
}

// Pattern is a rewrite with captures, for a shape a whole-word swap cannot
// express. Replace uses $1, $2 for the groups, as Go's regexp expansion spells
// it.
type Pattern struct {
	Match   string `xml:"match,attr"`
	Replace string `xml:"replace,attr"`
	Where   string `xml:"where,attr"`
	Test    string `xml:"test,attr"`
	Expect  string `xml:"expect,attr"`
	ID      string `xml:"id,attr"`
	Cases   []Case `xml:"test"`

	// Subject names where the clause's subject stands, and empty asks nothing.
	Subject string `xml:"subject,attr"`

	re *regexp.Regexp
}

type Case struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

func (p Pattern) Tests() []Case { return cases(p.Test, p.Expect, p.Cases) }

// Tests is every worked example a drop declares.
func (d Drop) Tests() []Case { return cases(d.Test, "", d.Cases) }

// Tests is every worked example a rewrite declares.
func (r Rewrite) Tests() []Case { return cases(r.Test, "", r.Cases) }

func cases(in, out string, extra []Case) []Case {
	var all []Case
	if in != "" {
		all = append(all, Case{In: in, Out: out})
	}
	return append(all, extra...)
}

// Apply rewrites every occurrence the pattern matches.
func (p Pattern) Apply(s string) string {
	out, _ := p.ApplyN(s)
	return out
}

// ApplyN is Apply, and the count of what it changed.
//
// A pattern naming a subject position rewrites only the matches whose clause
// states the sentence's own assertion. The rest are left whole: the words fit
// the shape, and the sentence around them says they report the data rather than
// the tree.
func (p Pattern) ApplyN(s string) (string, int) {
	var out strings.Builder
	last, took := 0, 0
	for _, loc := range p.re.FindAllStringIndex(s, -1) {
		if p.Subject != "" && !asserts(p.Subject, s, loc[0]) {
			continue
		}
		with := p.re.ReplaceAllString(s[loc[0]:loc[1]], p.Replace)
		if !keepsNegation(s, loc[0], loc[1], with) {
			continue
		}
		out.WriteString(s[last:loc[0]])
		out.WriteString(with)
		last = loc[1]
		took++
	}
	out.WriteString(s[last:])
	return out.String(), took
}

// negation finds a word that turns a clause into its opposite.
var negation = regexp.MustCompile(`(?i)\b(?:no longer|not|no|never|none|cannot|nobody|nothing|neither|nor|without)\b|n't\b`)

// keepsNegation reports whether the rewrite of s[from:to] to with keeps what
// each negation says. A cut can take a negation only with all the words it
// negates, up to the end of the sentence.
func keepsNegation(s string, from, to int, with string) bool {
	cut := s[from:to]
	found := negation.FindAllStringIndex(cut, -1)
	if len(found) <= len(negation.FindAllString(with, -1)) {
		return true
	}
	rest := s[to:]
	if end := strings.IndexAny(rest, ".!?"); end >= 0 {
		rest = rest[:end]
	}
	if strings.IndexFunc(rest, unicode.IsLetter) >= 0 {
		return false
	}
	final := found[len(found)-1]
	return strings.IndexFunc(cut[final[1]:], unicode.IsLetter) >= 0
}

// PatternIDs names every rule the table carries, in the order it carries them.
func PatternIDs() []string {
	var out []string
	for _, p := range loaded.Patterns {
		if p.ID != "" {
			out = append(out, p.ID)
		}
	}
	return out
}

// Shape is a rewrite over word classes, in table's match language, for a word that is filler only in some slots.
type Shape struct {
	Match   string `xml:"match,attr"`
	Replace string `xml:"replace,attr"`
	Where   string `xml:"where,attr"`
	Test    string `xml:"test,attr"`
	Expect  string `xml:"expect,attr"`
	ID      string `xml:"id,attr"`
	Cases   []Case `xml:"test"`

	terms table.Match
}

// Tests is every worked example a shape declares.
func (s Shape) Tests() []Case { return cases(s.Test, s.Expect, s.Cases) }

// ApplyN rewrites every place the shape fits in a single pass. The count is
// whether it changed anything, as the scan does not number its matches.
func (s Shape) ApplyN(text string) (string, int) {
	entry := []table.Rephrase{{ID: s.ID, Match: s.Match, Terms: s.terms, To: s.Replace}}
	next := table.Rephrasings(classes, nil, entry, text)
	if next == text {
		return text, 0
	}
	return next, 1
}

// Shapes is every class-aware rewrite the table carries.
func Shapes() []Shape { return loaded.Shapes }

// Drops is every word a repair deletes.
func Drops() []Drop { return loaded.Drops }

// Rewrites is every phrase a repair swaps.
func Rewrites() []Rewrite { return loaded.Rewrites }

// Patterns is every shape a repair rewrites.
func Patterns() []Pattern { return loaded.Patterns }

// Drop is a word a repair deletes.
type Drop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
	Cases []Case `xml:"test"`
}

type Rewrite struct {
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
	Cases []Case `xml:"test"`
}

// AppliesTo says whether an entry covers a surface.
func AppliesTo(where, surface string) bool {
	return where == "" || where == "both" || slices.Contains(strings.Fields(where), surface)
}

// loaded is the parsed table.
var loaded = mustLoad()

func mustLoad() Table {
	var e Table
	if err := xml.Unmarshal(table.Readable(englishXML), &e); err != nil {
		panic(fmt.Sprintf("english: english.xml does not parse: %v", err))
	}
	if len(e.Drops) == 0 || len(e.Rewrites) == 0 {
		panic("english: english.xml carries no drops or no rewrites")
	}
	for _, d := range e.Drops {
		if d.Word == "" || len(d.Tests()) == 0 {
			panic("english: a <drop> is missing its word or its test")
		}
	}
	for _, r := range e.Rewrites {
		if r.From == "" || r.To == "" || len(r.Tests()) == 0 {
			panic("english: a <rewrite> is missing from, to or test")
		}
	}
	for _, f := range e.Flags {
		if f.Phrase == "" || f.Say == "" || len(f.Tests()) == 0 {
			panic("english: a <flag> is missing phrase, say or test")
		}
	}
	for i := range e.Shapes {
		s := &e.Shapes[i]
		if s.Match == "" || len(s.Tests()) == 0 {
			panic("english: a <shape> is missing match or test")
		}
		terms, err := table.ParseMatch(s.Match)
		if err != nil {
			panic(fmt.Sprintf("english: <shape match=%q> does not parse: %v", s.Match, err))
		}
		s.terms = terms
	}
	for i := range e.Patterns {
		p := &e.Patterns[i]
		if p.Match == "" || len(p.Tests()) == 0 {
			panic("english: a <pattern> is missing match or test")
		}
		for _, c := range p.Tests() {
			if c.Out == "" {
				panic(fmt.Sprintf("english: <pattern match=%q> has a test with no expected output", p.Match))
			}
		}
		if p.Subject != "" && !slices.Contains(SubjectPositions(), p.Subject) {
			panic(fmt.Sprintf("english: <pattern match=%q> names the subject position %q, which is not one of %v",
				p.Match, p.Subject, SubjectPositions()))
		}
		re, err := regexp.Compile(p.Match)
		if err != nil {
			panic(fmt.Sprintf("english: <pattern match=%q> does not compile: %v", p.Match, err))
		}
		p.re = re
	}
	return e
}
