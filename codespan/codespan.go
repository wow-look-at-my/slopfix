// Package codespan finds the inline code spans of a line of markdown prose.
//
// A run of backticks opens a span when the byte before it is no word byte and
// the byte after it is no blank. The span closes at the next run of the same
// length. Any other run is a stray: a literal backtick, which a repair must
// not pair with the opener of a later span.
package codespan

// Find answers the byte range of each code span in s, backticks included, in
// the order they open.
func Find(s string) [][]int {
	spans, _ := scan(s)
	return spans
}

// Stray answers the offset of each backtick run in s that is no part of a span.
func Stray(s string) []int {
	_, stray := scan(s)
	return stray
}

func scan(s string) (spans [][]int, stray []int) {
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := runEnd(s, i)
		if close := closer(s, i, j); close >= 0 {
			spans = append(spans, []int{i, close})
			i = close
			continue
		}
		stray = append(stray, i)
		i = j
	}
	return spans, stray
}

func closer(s string, i, j int) int {
	if j >= len(s) || blank(s[j]) || (i > 0 && wordByte(s[i-1])) {
		return -1
	}
	n := j - i
	for k := j; k < len(s); {
		if s[k] != '`' {
			k++
			continue
		}
		l := runEnd(s, k)
		if l-k == n {
			return l
		}
		k = l
	}
	return -1
}

func runEnd(s string, i int) int {
	j := i
	for j < len(s) && s[j] == '`' {
		j++
	}
	return j
}

func blank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func wordByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
