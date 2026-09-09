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
	"strings"
)

//go:embed english.xml
var englishXML []byte

// Table mirrors english.xml.
type Table struct {
	Drops    []Drop    `xml:"drop"`
	Rewrites []Rewrite `xml:"rewrite"`
	Patterns []Pattern `xml:"pattern"`
	Flags    []Flag    `xml:"flag"`
}

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

	re *regexp.Regexp
}

// Case is one worked example: prose in, prose out. An entry carries as many as
// it needs, as <test in="..." out="..."/> children, and the attribute pair on
// the entry itself is the first of them.
type Case struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

// Tests is every worked example an entry declares, the attribute pair first.
func (p Pattern) Tests() []Case { return cases(p.Test, p.Expect, p.Cases) }

// Tests is every worked example a drop declares. A drop states no output: the
// assertion is that the word is gone.
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
func (p Pattern) Apply(s string) string { return p.re.ReplaceAllString(s, p.Replace) }

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

// Rewrite is a phrase a repair swaps for a shorter one.
type Rewrite struct {
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
	Cases []Case `xml:"test"`
}

// AppliesTo says whether an entry covers a surface. An empty where= means both,
// so an entry that says nothing about surface applies everywhere.
func AppliesTo(where, surface string) bool {
	return where == "" || where == "both" || where == surface
}

// loaded is the parsed table. A malformed table is a build the binary refuses
// to start, because a rule set that silently loses entries enforces nothing.
var loaded = mustLoad()

func mustLoad() Table {
	var e Table
	if err := xml.Unmarshal(englishXML, &e); err != nil {
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
		re, err := regexp.Compile(p.Match)
		if err != nil {
			panic(fmt.Sprintf("english: <pattern match=%q> does not compile: %v", p.Match, err))
		}
		p.re = re
	}
	return e
}
