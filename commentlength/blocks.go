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
// has to end somewhere: at the close of the brace its opening line left open, at
// the dedent below it, or at the blank line that separates it from whatever
// comes next.
package commentlength

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/source"
)

// maxCodeLines bounds how far the LINE WALK follows a block's code. It is a
// guard on a guess, and it has no counterpart in commentspan: the Go path takes
// an exact span from the parser and follows the node however far it runs.
const maxCodeLines = 40

// generatedMarker is the canonical generated-file header. commentspan skips a
// file carrying it, because nobody can act on a finding in generated code.
var generatedMarker = regexp.MustCompile(`^\s*(?://+|#+)\s*Code generated .* DO NOT EDIT\.$`)

// isGenerated reports the marker in the file's header, above the first line of
// code. commentspan looks for it above the package clause, which is that
// region for a Go file.
func isGenerated(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !startsComment(trimmed) {
			return false
		}
		if generatedMarker.MatchString(line) {
			return true
		}
	}
	return false
}

// blocks returns every comment block in the file, each with the code it
// documents measured beside it.
func blocks(filename, src string) []block {
	if !source.Supported(filename) {
		return nil
	}
	if isGenerated(splitLines(src)) {
		return nil
	}
	// Go is parsed rather than walked: this rule deletes comment text, so it
	// takes the exact span from go/ast and yields nothing on a parse failure.
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

// startsComment reports a line whose leading token opens a comment.
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
func measureCode(lines []string, from, indent int) (int, int) {
	depth := 0
	var code []string
	for i := from; i < len(lines) && len(code) < maxCodeLines; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || startsComment(trimmed) {
			// A blank line or a fresh comment ends the span, unless a brace is still open.
			if depth <= 0 && len(code) > 0 {
				break
			}
			continue
		}
		if len(code) > 0 && depth <= 0 && indentOf(line) <= indent && closesNothing(lines[from]) {
			break
		}

		code = append(code, line)
		depth += braceDelta(trimmed)
		if depth <= 0 && opensBrace(lines[from]) {
			break
		}
	}
	// The same measure the comment gets, so both counts compare directly.
	return measure(code)
}

// closesNothing reports an opening line that opened no brace, so indentation bounds the span.
func closesNothing(first string) bool { return !opensBrace(first) }

// opensBrace reports a line that leaves a brace open.
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

// indentOf is a line's leading whitespace width, with a tab counted as a level.
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
