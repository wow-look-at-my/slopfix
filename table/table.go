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
// repair must produce.
type Test struct {
	In  string
	Out string
}

// A Detect states what a single substrate reports for a line. An empty Found
// says it reports nothing.
type Detect struct {
	In        string
	Substrate string
	Why       string
	Found     []string
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

type Table struct {
	Drops       []Drop
	Rewrites    []Rewrite
	Patterns    []Pattern
	Flags       []Flag
	Classes     []Class
	Normals     []Normalize
	Rephrasings []Rephrase

	// Tests are the worked examples the folder states for the consumer rather than for a single entry: a line of prose,
	Tests []Test

	// Detects are what each substrate reports, for the reader rather than the repair.
	Detects []Detect

	once    sync.Once
	lexicon *Lexicon
}

// Lexicon indexes the table's word classes. It is built on the earliest read
// and kept, because a shape asks it for every word of every comment.
func (t *Table) Lexicon() *Lexicon {
	t.once.Do(func() { t.lexicon = NewLexicon(t.Classes) })
	return t.lexicon
}

// WordsOf answers the words a named class lists, so a caller that needs the
// members themselves rather than a membership test reads them.
func (t *Table) WordsOf(class string) []string {
	for _, c := range t.Classes {
		if c.Name == class {
			return c.Words
		}
	}
	return nil
}

// AppliesTo reports whether an entry's where= covers a surface, and an empty
// value means both.
func AppliesTo(where, surface string) bool {
	return where == "" || where == "both" || where == surface
}

// Replace rewrites every match of the pattern in s, and returns s untouched
// when there is none.
func (p Pattern) Replace(s string) string {
	return p.re.ReplaceAllStringFunc(s, func(matched string) string {
		at := p.re.FindStringSubmatchIndex(matched)
		if at == nil {
			return matched
		}
		return MatchCase(matched, string(p.re.ExpandString(nil, p.To, matched, at)))
	})
}

// MatchCase gives a replacement the opening case of the text it replaces.
//
// A table matches without regard to case, so a rule that writes its
// replacement as it stands lowercases the word that opens a sentence. A reader
// then finds a sentence starting in the middle of a line.
func MatchCase(matched, replacement string) string {
	if matched == "" || replacement == "" {
		return replacement
	}
	if head := matched[0]; head < 'A' || head > 'Z' {
		return replacement
	}
	if first := replacement[0]; first >= 'a' && first <= 'z' {
		return string(first-'a'+'A') + replacement[1:]
	}
	return replacement
}

// Matches reports whether the pattern finds anything in s.
func (p Pattern) Matches(s string) bool { return p.re.MatchString(s) }
