// cmake.go reads a CMake listfile the way cmake-language(7) does.
//
// The bash fallback finds the comments of a hash-comment file. CMake spells
// things bash does not know: a bracket argument and a bracket comment. A line
// inside either opens on `#` and is still no line comment. The lexer here
// names the line comments CMake reads, and the tokens a repair must keep.
package treecomments

import (
	"path/filepath"
	"slices"
	"strings"
)

// IsCMake reports a CMake listfile: a CMakeLists.txt, or a script or module
// that ends in .cmake.
func IsCMake(filename string) bool {
	base := strings.ToLower(filepath.Base(filename))
	return base == "cmakelists.txt" || filepath.Ext(base) == ".cmake"
}

// cmakeLexed is a listfile cut into what CMake reads.
type cmakeLexed struct {
	// lineComments holds where each line comment starts and ends.
	lineComments []span
	// tokens holds every argument, parenthesis and command name, in order. Whitespace and comments are no token.
	tokens []string
}

// lexCMake cuts src into line comments and tokens.
func lexCMake(src string) cmakeLexed {
	var out cmakeLexed
	for at := 0; at < len(src); {
		c := src[at]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			at++
		case c == '#':
			if end, ok := bracketEnd(src, at+1); ok {
				at = end
				continue
			}
			end := strings.IndexByte(src[at:], '\n')
			if end < 0 {
				end = len(src) - at
			}
			out.lineComments = append(out.lineComments, span{start: at, end: at + end})
			at += end
		case c == '(' || c == ')':
			out.tokens = append(out.tokens, string(c))
			at++
		case c == '"':
			end := quotedEnd(src, at+1)
			out.tokens = append(out.tokens, src[at:end])
			at = end
		case c == '[':
			if end, ok := bracketEnd(src, at); ok {
				out.tokens = append(out.tokens, src[at:end])
				at = end
				continue
			}
			end := unquotedEnd(src, at)
			out.tokens = append(out.tokens, src[at:end])
			at = end
		default:
			end := unquotedEnd(src, at)
			out.tokens = append(out.tokens, src[at:end])
			at = end
		}
	}
	return out
}

// bracketEnd answers where the bracket that opens at src[at] closes: past
// `]=*]` with as many `=` as the opener. ok is false when src[at] opens none.
// A bracket that never closes runs to the end of src, as CMake reads it.
func bracketEnd(src string, at int) (int, bool) {
	if at >= len(src) || src[at] != '[' {
		return 0, false
	}
	level := 0
	for at+1+level < len(src) && src[at+1+level] == '=' {
		level++
	}
	if at+1+level >= len(src) || src[at+1+level] != '[' {
		return 0, false
	}
	closer := "]" + strings.Repeat("=", level) + "]"
	body := at + 2 + level
	end := strings.Index(src[body:], closer)
	if end < 0 {
		return len(src), true
	}
	return body + end + len(closer), true
}

// quotedEnd answers where a quoted argument whose body starts at at closes,
// past its closing quote. A backslash escapes the byte after it.
func quotedEnd(src string, at int) int {
	for at < len(src) {
		switch src[at] {
		case '\\':
			at += 2
		case '"':
			return at + 1
		default:
			at++
		}
	}
	return len(src)
}

// unquotedEnd answers where an unquoted argument that starts at at stops. A
// backslash escapes the byte after it, a `#` among them.
func unquotedEnd(src string, at int) int {
	for at < len(src) {
		switch src[at] {
		case ' ', '\t', '\r', '\n', '(', ')', '#', '"':
			return at
		case '\\':
			at += 2
		default:
			at++
		}
	}
	return len(src)
}

// cmakeLineComments keeps each comment that is a whole CMake line comment.
// The bash fallback reads a line inside a bracket argument or a bracket
// comment as a comment, and CMake does not.
func cmakeLineComments(src string, comments []Comment) []Comment {
	real := lexCMake(src).lineComments
	kept := comments[:0:0]
	for _, c := range comments {
		end := c.Offset + len(strings.TrimRight(c.Text, "\r"))
		if slices.ContainsFunc(real, func(s span) bool {
			return s.start == c.Offset && strings.TrimRight(src[s.start:s.end], "\r") == src[c.Offset:end]
		}) {
			kept = append(kept, c)
		}
	}
	return kept
}

// sameCMake reports whether text holds every token src holds, in order. An
// edit that keeps them changed no command CMake runs.
func sameCMake(src, text string) bool {
	return slices.Equal(lexCMake(src).tokens, lexCMake(text).tokens)
}
