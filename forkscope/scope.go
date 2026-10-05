package forkscope

import (
	"errors"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/wow-look-at-my/go-containers/set"
)

// Scope is the lines of one file that the fork wrote, counted from one.
type Scope struct {
	whole bool
	lines set.Set[int]
	// base reads the file as the fork's base has it, the text the lines are measured against.
	base func() (string, error)
}

// errNoBase answers Base for a Scope built from line numbers alone.
var errNoBase = errors.New("fork scope: no base text is known for this file")

// Base answers the file as the fork's base has it.
func (s *Scope) Base() (string, error) {
	if s.base == nil {
		return "", errNoBase
	}
	return s.base()
}

// Whole is the Scope of a file the fork wrote all of.
func Whole() *Scope { return &Scope{whole: true} }

// OfLines is the Scope of the named lines.
func OfLines(lines ...int) *Scope { return &Scope{lines: set.Of(lines...)} }

// All reports whether the fork wrote every line.
func (s *Scope) All() bool { return s.whole }

// Empty reports whether the fork wrote no line.
func (s *Scope) Empty() bool { return !s.whole && s.lines.Len() == 0 }

// Owns reports whether the fork wrote line n.
func (s *Scope) Owns(n int) bool { return s.whole || s.lines.Contains(n) }

// Holds reports whether the fork wrote any line from first to last. A line
// number below one stands for the file, which the fork wrote when it wrote any
// line of it.
func (s *Scope) Holds(first, last int) bool {
	if s.whole {
		return true
	}
	if first < 1 {
		return s.lines.Len() > 0
	}
	for n := first; n <= max(first, last); n++ {
		if s.lines.Contains(n) {
			return true
		}
	}
	return false
}

// Widen answers s with every line of each block of text that holds a line s
// names. In a document, a block is a paragraph or a list item. In other text,
// a block is a run of comment lines. A rule judges a block whole, so the fork
// owns all of a block it wrote into.
func (s *Scope) Widen(text string, document bool) *Scope {
	if s.whole {
		return s
	}
	blocks := blocksOf(text, document)
	reach := set.New[int]()
	for n := range s.lines.All() {
		if n >= 1 && n <= len(blocks) && blocks[n-1] >= 0 {
			reach.Add(blocks[n-1])
		}
	}
	out := &Scope{lines: set.New[int](), base: s.base}
	for n := range s.lines.All() {
		out.lines.Add(n)
	}
	for i, b := range blocks {
		if b >= 0 && reach.Contains(b) {
			out.lines.Add(i + 1)
		}
	}
	return out
}

// blocksOf answers the block of each line of text, counted from zero.
func blocksOf(text string, document bool) []int {
	rows := strings.Split(text, "\n")
	out := make([]int, len(rows))
	block, open, fenced := -1, false, false
	for i, row := range rows {
		trimmed := strings.TrimSpace(row)
		inBlock, starts := false, false
		switch {
		case document && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			fenced = !fenced
		case document && !fenced:
			inBlock = trimmed != "" && !strings.HasPrefix(trimmed, "|")
			starts = strings.HasPrefix(trimmed, "#") || opensItem(trimmed)
		case !document:
			inBlock = isCommentRow(trimmed)
		}
		if !inBlock {
			out[i], open = -1, false
			continue
		}
		if !open || starts {
			block++
		}
		out[i], open = block, !strings.HasPrefix(trimmed, "#") || !document
	}
	return out
}

// opensItem reports whether a trimmed markdown line opens a list item.
func opensItem(trimmed string) bool {
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	digits := len(trimmed) - len(strings.TrimLeft(trimmed, "0123456789"))
	rest := trimmed[digits:]
	return digits > 0 && (strings.HasPrefix(rest, ". ") || strings.HasPrefix(rest, ") "))
}

// isCommentRow reports whether a trimmed source line is a comment and nothing else.
func isCommentRow(trimmed string) bool {
	for _, marker := range []string{"//", "#", "/*", "*", "--", ";"} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	return false
}

// Keep answers after with every change to a line the fork did not write put
// back as before had it. A change is a run of lines a line diff pairs up. It
// lands when the fork wrote every line it replaces. A run of new lines between
// old ones lands when the fork wrote a line beside it.
func Keep(before, after string, s *Scope) string {
	if s.whole || before == after {
		return after
	}
	a, b := splitLines(before), splitLines(after)
	var out strings.Builder
	for _, op := range opcodes(before, after) {
		if op.Tag == 'e' || !s.mayChange(op.I1, op.I2) {
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
			continue
		}
		out.WriteString(strings.Join(b[op.J1:op.J2], ""))
	}
	return out.String()
}

// mayChange reports whether the fork wrote the lines from i1 up to i2,
// counted from zero.
func (s *Scope) mayChange(i1, i2 int) bool {
	if i1 == i2 {
		return s.Owns(i1) || s.Owns(i1+1)
	}
	for i := i1; i < i2; i++ {
		if !s.Owns(i + 1) {
			return false
		}
	}
	return true
}

// Carry answers the lines of after that the fork wrote, where s names those of
// before. A line after keeps from before keeps its owner. A line after changed
// is the fork's.
func Carry(before, after string, s *Scope) *Scope {
	if s.whole {
		return Whole()
	}
	out := &Scope{}
	for _, op := range opcodes(before, after) {
		for j := op.J1; j < op.J2; j++ {
			if op.Tag != 'e' || s.Owns(op.I1+j-op.J1+1) {
				out.lines.Add(j + 1)
			}
		}
	}
	return out
}

// Hunk is a change after makes to base that leaves more lines than.
type Hunk struct{ J1, J2, Had int }

// Grown answers each change after makes to base that reaches into rows first
// to last of after, counted from one, and leaves more lines than.
func Grown(base, after string, first, last int) []Hunk {
	var out []Hunk
	for _, op := range opcodes(base, after) {
		if op.Tag == 'e' || op.J2-op.J1 <= op.I2-op.I1 || op.J2 < first || op.J1 >= last {
			continue
		}
		out = append(out, Hunk{J1: op.J1, J2: op.J2, Had: op.I2 - op.I1})
	}
	return out
}

// Removed answers the text of every line before had that after replaced or dropped.
func Removed(before, after string) string {
	a := splitLines(before)
	var out strings.Builder
	for _, op := range opcodes(before, after) {
		if op.Tag == 'r' || op.Tag == 'd' {
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
		}
	}
	return out.String()
}

// opcodes answers the line diff from before to after.
func opcodes(before, after string) []difflib.OpCode {
	return difflib.NewMatcherWithJunk(splitLines(before), splitLines(after), false, nil).GetOpCodes()
}

// splitLines cuts s after each line end. The pieces join back to s.
func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
