// blocks.go pairs a run of comment lines with the code beneath it.
//
// It walks lines rather than parsing the language, and that is a tradeoff
// rather than a necessity. Go's own go/ast is in the standard library and
// would answer exactly this question for a .go file, and tree-sitter grammars
// run cgo-free under a wasm runtime. What the walk buys is that every language
// the source adapter spells is covered by the same code the day the adapter
// learns it, with no grammar to ship per language. Reach for a real parser
// here the moment the span this computes is wrong often enough to matter.
//
// What the walk has to get right is narrow. A comment marker inside a string
// is data, so the source adapter decides which text is really a comment and
// this file only reads the lines it names. And the code a comment documents
// has to end somewhere: at the close of the brace the first line opened, at
// the dedent below it, or at the blank line that separates it from whatever
// comes next.
package commentlength

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/source"
)

// maxCodeLines bounds how far a block's code is followed. Past this the
// a long function is judged against the function's opening, which is what a
// reader actually holds in their head while reading it.
const maxCodeLines = 40

// blocks returns every comment block in the file, each with the code it
// documents measured beside it.
func blocks(filename, src string) []block {
	if !source.Supported(filename) {
		return nil
	}
	// Go is parsed rather than walked. This rule deletes comment text, so the
	// language this org writes most gets the exact span from go/ast, and a file
	// that does not parse yields nothing at all.
	if strings.HasSuffix(strings.ToLower(filename), ".go") {
		parsed, ok := goBlocks(src)
		if !ok {
			return nil
		}
		return parsed
	}
	lines := splitLines(src)
	comment := commentLines(filename, src, len(lines))

	var out []block
	for i := 0; i < len(lines); {
		if !comment[i] {
			i++
			continue
		}
		start := i
		for i < len(lines) && comment[i] {
			i++
		}
		b := block{start: start, end: i, text: lines[start:i]}
		b.codeLines, b.codeChars = measureCode(lines, i, indentOf(lines[start]))
		out = append(out, b)
	}
	return out
}

// commentLines marks the lines the source adapter reports as comment text.
//
// A line is a comment line only when the comment is the whole of it. A
// trailing comment on a code line documents nothing of its own, and counting
// it would measure a statement against the note beside it.
func commentLines(filename, src string, n int) []bool {
	out := make([]bool, n)
	lines := splitLines(src)
	for _, c := range source.Extract(filename, src) {
		first := lineAt(src, c.Offset)
		for k := range strings.Count(c.Text, "\n") + 1 {
			at := first + k
			if at < 0 || at >= n {
				continue
			}
			out[at] = true
		}
	}
	for i, line := range lines {
		if !out[i] {
			continue
		}
		if t := strings.TrimSpace(line); t == "" || !startsComment(t) {
			out[i] = false
		}
	}
	return out
}

// startsComment reports a line whose first token opens a comment. The markers
// are the ones the source adapter's syntax table spells, plus the leading `*`
// a wrapped block comment conventionally carries.
func startsComment(trimmed string) bool {
	for _, marker := range []string{"//", "/*", "#", "*/", "*"} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	return false
}

// lineAt is the line index holding a byte offset.
func lineAt(src string, offset int) int {
	if offset < 0 || offset > len(src) {
		return 0
	}
	return strings.Count(src[:offset], "\n")
}

// measureCode measures the code a block documents, starting at line from.
//
// The span ends at whichever comes first: the close of the brace its opening
// line left open, a line indented no deeper than the comment that is blank or
// starts a new comment, or the bound above. A language with no braces falls
// through to the indentation rule, which is what makes this work on Bash and
// Python as well as on Go.
func measureCode(lines []string, from, indent int) (int, int) {
	depth := 0
	count, chars := 0, 0
	for i := from; i < len(lines) && count < maxCodeLines; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || startsComment(trimmed) {
			// A blank line or a fresh comment ends the span, but only once the
			// braces it opened are closed: a blank line inside a function body
			// is part of that body.
			if depth <= 0 && count > 0 {
				break
			}
			continue
		}
		if count > 0 && depth <= 0 && indentOf(line) <= indent && closesNothing(lines[from]) {
			break
		}

		count++
		chars += len(trimmed)
		depth += braceDelta(trimmed)
		if count > 0 && depth <= 0 && opensBrace(lines[from]) {
			break
		}
	}
	return count, chars
}

// closesNothing reports a first line that opened no brace, so the span is
// bounded by indentation rather than by a matching close.
func closesNothing(first string) bool { return !opensBrace(first) }

// opensBrace reports a line that leaves a brace open, which is how a C-family
// declaration announces that its body follows.
func opensBrace(line string) bool { return braceDelta(strings.TrimSpace(line)) > 0 }

// braceDelta counts the braces a line opens less the ones it closes, ignoring
// any inside a string or a character literal.
func braceDelta(line string) int {
	delta, quote := 0, byte(0)
	for i := range len(line) {
		c := line[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '{' || c == '(' || c == '[':
			delta++
		case c == '}' || c == ')' || c == ']':
			delta--
		}
	}
	return delta
}

// indentOf is a line's leading whitespace width, with a tab counted as a
// single level so a tab-indented file and a space-indented one compare the
// same way against their own neighbours.
func indentOf(line string) int {
	n := 0
	for i := range len(line) {
		switch line[i] {
		case ' ', '\t':
			n++
		default:
			return n
		}
	}
	return n
}
