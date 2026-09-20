// Package table is the prose tables in rules/, as the binary holds them.
//
// A rules file is a Table, so the XML unmarshals straight into the types a
// consumer reads. Load takes the embedded folder and hands back what a single
// `for` value adds up to, with every pattern compiled.
package table

<<<<<<< HEAD
import "regexp"

// Drop is a word that survives its own deletion.
type Drop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
=======
import (
	"strings"
	"sync"
)

// A Test drives an entry. In is the prose somebody writes. Out is what the
// repair must produce. An entry that asserts no particular output leaves Out
// empty, and the test then only holds the entry to firing at all.
type Test struct {
	In  string
	Out string
}

// Drop is a word that survives its own deletion.
type Drop struct {
	ID    string
	Word  string
	Where string
	Tests []Test
>>>>>>> origin/master
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
<<<<<<< HEAD
	From   string `xml:"from,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
=======
	ID    string
	From  string
	To    string
	Where string
	Tests []Test
>>>>>>> origin/master
}

// Flag names prose a rule refuses to rewrite, and what to write instead.
type Flag struct {
<<<<<<< HEAD
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
=======
	ID     string
	Phrase string
	Say    string
	Tests  []Test
>>>>>>> origin/master
}

// Pattern is a rewrite with captures.
type Pattern struct {
<<<<<<< HEAD
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
=======
	ID    string
	Match string
	To    string
	Where string
	Tests []Test
>>>>>>> origin/master

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
<<<<<<< HEAD
	For      string    `xml:"for,attr"`
	Drops    []Drop    `xml:"drop"`
	Rewrites []Rewrite `xml:"rewrite"`
	Patterns []Pattern `xml:"pattern"`
	Flags    []Flag    `xml:"flag"`
	Cases    []Case    `xml:"case"`
=======
	Drops       []Drop
	Rewrites    []Rewrite
	Patterns    []Pattern
	Flags       []Flag
	Classes     []Class
	Normals     []Normalize
	Rephrasings []Rephrase

	once    sync.Once
	lexicon *Lexicon
}

// Lexicon indexes the table's word classes. It is built on the earliest read
// and kept, because a shape asks it for every word of every comment.
func (t *Table) Lexicon() *Lexicon {
	t.once.Do(func() { t.lexicon = NewLexicon(t.Classes) })
	return t.lexicon
>>>>>>> origin/master
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
