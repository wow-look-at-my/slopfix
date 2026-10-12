// Package goformat puts a repaired Go file back in the layout gofmt writes.
//
// A cut comment can leave a blank line too many, or bring together fields that
// gofmt aligns. The gate admits blanks for blanks, and the Go scanner must then
// read the tokens the file held.
package goformat

import (
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
)

// Name is the fixer's name in the registry.
const Name = "gofmt"

// Edits answers the whitespace edits that give src the gofmt layout. A fragment
// with no package clause, source that does not parse, and a layout that changes
// more than whitespace get none.
func Edits(src string) []edit.Edit {
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly); err != nil {
		return nil
	}
	out, err := format.Source([]byte(src))
	if err != nil || string(out) == src {
		return nil
	}
	want := string(out)
	var edits []edit.Edit
	i, j := 0, 0
	for {
		from, to := i, j
		i, j = skipBlank(src, i), skipBlank(want, j)
		if src[from:i] != want[to:j] {
			edits = append(edits, edit.Edit{Start: from, End: i, Text: want[to:j]})
		}
		if i == len(src) || j == len(want) {
			break
		}
		if src[i] != want[j] {
			return nil
		}
		i++
		j++
	}
	if i != len(src) || j != len(want) {
		return nil
	}
	return edits
}

// skipBlank answers where the blanks at s[i] end.
func skipBlank(s string, i int) int {
	for {
		for i < len(s) && blank(s[i]) {
			i++
		}
		if !bareComment(s, i) {
			return i
		}
		i += len("//")
	}
}

// bareComment reports a "//" at s[i] that is the whole of its line.
func bareComment(s string, i int) bool {
	if !strings.HasPrefix(s[i:], "//") {
		return false
	}
	if end := i + len("//"); end < len(s) && s[end] != '\n' && s[end] != '\r' {
		return false
	}
	k := i
	for k > 0 && (s[k-1] == ' ' || s[k-1] == '\t') {
		k--
	}
	return k == 0 || s[k-1] == '\n'
}

// Gate writes an edit only when it trades blanks for blanks. It holds only when
// the scanner reads the tokens src holds, so a blank inside a literal or a
// directive never moves.
func Gate(src string, edits []edit.Edit, scope edit.Scope) edit.Result {
	want := tokens(src)
	reach := func(e edit.Edit) string {
		if skipBlank(src, e.Start) < e.End || skipBlank(e.Text, 0) != len(e.Text) {
			return "it changes more than whitespace"
		}
		return ""
	}
	holds := func(text string) bool { return slices.Equal(tokens(text), want) }
	return edit.Gate(src, edits, scope, reach, holds)
}

// tokens answers each token the scanner reads in src, with its text. A comment
// loses the blanks at the end of each of its lines, because gofmt trims them.
func tokens(src string) []string {
	file := token.NewFileSet().AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), nil, scanner.ScanComments)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.COMMENT {
			lit = trimLineEnds(lit)
			// A bare marker line is a blank, so gofmt may add or drop it.
			if lit == "//" {
				continue
			}
		}
		out = append(out, tok.String()+" "+lit)
	}
}

func trimLineEnds(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.Join(lines, "\n")
}

// blank is the whitespace the Go scanner skips.
func blank(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
