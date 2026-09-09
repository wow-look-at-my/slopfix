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

	re *regexp.Regexp
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

// Flags is every phrase the table refuses to rewrite.
func Flags() []Flag { return loaded.Flags }

// Drop is a word a repair deletes.
type Drop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

// Rewrite is a phrase a repair swaps for a shorter one.
type Rewrite struct {
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

// AppliesTo says whether an entry covers a surface. An empty where= means both,
// so an entry that says nothing about surface applies everywhere.
func AppliesTo(where, surface string) bool {
	return where == "" || where == "both" || where == surface
}

// Flag is prose the table knows is bad and will not rewrite.
type Flag struct {
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
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
		if d.Word == "" || d.Test == "" {
			panic("english: a <drop> is missing its word or its test")
		}
	}
	for _, r := range e.Rewrites {
		if r.From == "" || r.To == "" || r.Test == "" {
			panic("english: a <rewrite> is missing from, to or test")
		}
	}
	for _, f := range e.Flags {
		if f.Phrase == "" || f.Say == "" || f.Test == "" {
			panic("english: a <flag> is missing phrase, say or test")
		}
	}
	for i := range e.Patterns {
		p := &e.Patterns[i]
		if p.Match == "" || p.Test == "" || p.Expect == "" {
			panic("english: a <pattern> is missing match, test or expect")
		}
		re, err := regexp.Compile(p.Match)
		if err != nil {
			panic(fmt.Sprintf("english: <pattern match=%q> does not compile: %v", p.Match, err))
		}
		p.re = re
	}
	return e
}

