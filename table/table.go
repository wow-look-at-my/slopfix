// Package table is the prose tables in rules/, as the binary holds them.
//
// The folder is embedded and read at start-up: Load hands back what a single
// `for` value adds up to, with every pattern compiled and every word class
// indexed. This package holds the entry types and the matchers that run them.
package table

import (
	"regexp"
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
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
	ID    string
	From  string
	To    string
	Where string
	Tests []Test
}

// Flag names prose a rule refuses to rewrite, and what to write instead.
type Flag struct {
	ID     string
	Phrase string
	Say    string
	Tests  []Test
}

// Pattern is a rewrite with captures.
type Pattern struct {
	ID    string
	Match string
	To    string
	Where string
	Tests []Test

	// re is Match, compiled when the folder is loaded.
	re *regexp.Regexp
}

// Table is what a single `for` value in rules/ adds up to.
type Table struct {
	Drops       []Drop
	Rewrites    []Rewrite
	Patterns    []Pattern
	Flags       []Flag
	Classes     []Class
	Normals     []Normalize
	Rephrasings []Rephrase

	// Tests are the worked examples the folder states for the consumer rather
	// than for a single entry: a line of prose, and what the consumer as a
	// whole writes for it. It is where prose reaching several entries, or
	// reaching none, is stated.
	Tests []Test

	once    sync.Once
	lexicon *Lexicon
}

// Lexicon indexes the table's word classes. It is built on the earliest read
// and kept, because a shape asks it for every word of every comment.
func (t *Table) Lexicon() *Lexicon {
	t.once.Do(func() { t.lexicon = NewLexicon(t.Classes) })
	return t.lexicon
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

// Matches reports whether the pattern finds anything in s. A caller asks this
// when the answer decides something other than the rewrite itself.
func (p Pattern) Matches(s string) bool { return p.re.MatchString(s) }
