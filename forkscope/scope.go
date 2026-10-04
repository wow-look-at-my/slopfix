package forkscope

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/wow-look-at-my/go-containers/set"
)

// Scope is the lines of one file that the fork wrote, counted from one.
type Scope struct {
	whole bool
	lines set.Set[int]
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
