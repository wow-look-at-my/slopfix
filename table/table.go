// Package table is the prose tables in rules/, as the binary holds them.
//
// A rules file is a Table, so the XML unmarshals straight into the types a
// consumer reads. Load takes the embedded folder and hands back what a single
// `for` value adds up to, with every pattern compiled.
package table

import "regexp"

// Drop is a word that survives its own deletion.
type Drop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
	From   string `xml:"from,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
}

// Flag names prose a rule refuses to rewrite, and what to write instead.
type Flag struct {
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
}

// Pattern is a rewrite with captures.
type Pattern struct {
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`

	// re is Match, compiled when the folder is loaded.
	re *regexp.Regexp
}

// Case is a phrase and what the consumer must write for it. An entry's own
// test drives that entry alone. A case drives the whole consumer, which is
// where a phrase that several entries reach, or that none reach, is stated.
type Case struct {
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
}

// Table is what a single `for` value in rules/ adds up to. It is also the shape
// of a single rules file, so the folder loads into it a file at a time.
type Table struct {
	For      string    `xml:"for,attr"`
	Drops    []Drop    `xml:"drop"`
	Rewrites []Rewrite `xml:"rewrite"`
	Patterns []Pattern `xml:"pattern"`
	Flags    []Flag    `xml:"flag"`
	Cases    []Case    `xml:"case"`
}

// AppliesTo reports whether an entry's where= covers a surface, and an empty
// value means both.
func AppliesTo(where, surface string) bool {
	return where == "" || where == "both" || where == surface
}

// Replace rewrites every match of the pattern in s, and returns s untouched
// when there is none.
func (p Pattern) Replace(s string) string {
	return p.re.ReplaceAllString(s, p.To)
}
