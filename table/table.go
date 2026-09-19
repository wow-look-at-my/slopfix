// Package table is the run-time half of the prose tables in rules/.
//
// The XML in rules/ is build-time input. cmd/rulegen parses it, compiles every
// pattern with go-regex-compiler, and writes Go. This package holds what that
// generated Go is made of: the entry types, and the driver that walks a string
// with the compiled matchers.
//
// Nothing here parses XML or compiles a regexp. The binary carries a switch
// automaton per pattern and the table as a slice literal.
package table

import (
	"strings"
	"sync"
)

// Drop is a word that survives its own deletion.
type Drop struct {
	Word  string
	Where string
	Test  string
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
	From   string
	To     string
	Where  string
	Test   string
	Expect string
}

// Flag names prose a rule refuses to rewrite, and what to write instead.
type Flag struct {
	Phrase string
	Say    string
	Test   string
}

// Pattern is a rewrite with captures, compiled to Go.
//
// Has, At and Find are generated. Has answers the whole string in a single
// pass, At answers the head of a string, and both allocate nothing: a run over
// prose carrying no match therefore allocates nothing at all.
type Pattern struct {
	Match  string
	To     string
	Where  string
	Test   string
	Expect string

	// Has reports a match anywhere in s.
	Has func(s string) bool
	// At reports a match starting at the head of s.
	At func(s string) bool
	// Find returns the submatch index slice for the match at the head of s.
	Find func(s string) []int
	// LeadWord and TailWord carry a \b rulegen stripped off the pattern.
	LeadWord bool
	TailWord bool
}

// Table is what a single `for` value in rules/ adds up to.
type Table struct {
	Drops    []Drop
	Rewrites []Rewrite
	Patterns []Pattern
	Flags    []Flag
	Classes  []Class
	Normals  []Normalize
	Rephrasings []Rephrase

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

// Replace rewrites every match of the pattern in s, the way
// regexp.ReplaceAllString does, and returns s untouched when there is none.
func (p Pattern) Replace(s string) string {
	if p.Has == nil || !p.Has(s) {
		return s
	}
	var out strings.Builder
	last, i := 0, 0
	for i <= len(s) {
		start, end, groups, ok := p.match(s, i)
		if !ok {
			i++
			continue
		}
		out.WriteString(s[last:start])
		out.WriteString(expand(p.To, s[i:], groups))
		last = end
		i = end
		if start == end {
			// An empty match advances by a byte, or the loop never ends.
			if end < len(s) {
				out.WriteByte(s[end])
				last = end + 1
			}
			i = end + 1
		}
	}
	out.WriteString(s[last:])
	return out.String()
}

// match answers the span of a match starting at i, with the word boundaries
// the compiled matcher could not carry applied.
func (p Pattern) match(s string, i int) (start, end int, groups []int, ok bool) {
	if !p.At(s[i:]) {
		return 0, 0, nil, false
	}
	if p.LeadWord && i > 0 && isWordByte(s[i-1]) {
		return 0, 0, nil, false
	}
	idx := p.Find(s[i:])
	if len(idx) < 4 || idx[2] < 0 {
		return 0, 0, nil, false
	}
	start, end = i+idx[2], i+idx[3]
	if p.TailWord && end < len(s) && isWordByte(s[end]) {
		return 0, 0, nil, false
	}
	return start, end, idx, true
}

// expand writes the replacement, resolving a dollar group reference against
// the captures. A source group sits a step along, because the whole match
// takes the earliest slot.
func expand(to, s string, idx []int) string {
	if !strings.ContainsRune(to, '$') {
		return to
	}
	var out strings.Builder
	for i := 0; i < len(to); i++ {
		if to[i] != '$' || i+1 >= len(to) {
			out.WriteByte(to[i])
			continue
		}
		group, width, ok := groupRef(to[i+1:])
		if !ok {
			out.WriteByte(to[i])
			continue
		}
		i += width
		at := 2 * (group + 1)
		if at+1 < len(idx) && idx[at] >= 0 {
			out.WriteString(s[idx[at]:idx[at+1]])
		}
	}
	return out.String()
}

// groupRef reads the group number after a dollar, in either spelling, and
// answers how many bytes it spent.
func groupRef(rest string) (group, width int, ok bool) {
	body, braced := rest, false
	if rest[0] == '{' {
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			return 0, 0, false
		}
		body, braced = rest[1:end], true
	}
	digits := 0
	for digits < len(body) && body[digits] >= '0' && body[digits] <= '9' {
		group = group*10 + int(body[digits]-'0')
		digits++
	}
	if digits == 0 || (braced && digits != len(body)) {
		return 0, 0, false
	}
	if braced {
		return group, digits + 2, true
	}
	return group, digits, true
}

// isWordByte reports the character class a regexp word boundary reads.
func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
