package bashclean

import "strings"

// posixToOnig rewrites a POSIX basic or extended pattern for Oniguruma. In a
// basic pattern, a bare `( ) { } | + ?` is literal and the escaped form is the
// operator. Oniguruma reads them the other way around.
func posixToOnig(p string, basic bool) (string, bool) {
	var b strings.Builder
	atStart := true
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '[':
			n, ok := bracket(p[i:], &b)
			if !ok {
				return "", false
			}
			i += n - 1
			atStart = false
			continue
		case c == '\\':
			if i+1 >= len(p) {
				return "", false
			}
			i++
			e := p[i]
			switch {
			case e == '<' || e == '>':
				b.WriteString(`\b`)
			case basic && strings.IndexByte("(){}|+?", e) >= 0:
				b.WriteByte(e)
				atStart = e == '(' || e == '|'
				continue
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
		case basic && strings.IndexByte("(){}|+?", c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '*' && atStart:
			b.WriteString(`\*`)
		case basic && c == '^' && !atStart:
			b.WriteString(`\^`)
		case basic && c == '$' && !endsGroup(p[i+1:]):
			b.WriteString(`\$`)
		default:
			b.WriteByte(c)
			if (!basic && (c == '(' || c == '|')) || (c == '^' && atStart) {
				atStart = true
				continue
			}
		}
		atStart = false
	}
	return b.String(), true
}

// endsGroup asks whether a `$` sits where a basic pattern reads it as an
// anchor: at the end, or before `\)` or `\|`.
func endsGroup(rest string) bool {
	return rest == "" || strings.HasPrefix(rest, `\)`) || strings.HasPrefix(rest, `\|`)
}

// bracket copies a POSIX bracket expression and returns its length. A
// backslash, a nested `[` and `&` are literal in POSIX and special in
// Oniguruma, so each gets a backslash.
func bracket(p string, b *strings.Builder) (int, bool) {
	b.WriteByte('[')
	i := 1
	if i < len(p) && p[i] == '^' {
		b.WriteByte('^')
		i++
	}
	if i < len(p) && p[i] == ']' {
		b.WriteString(`\]`)
		i++
	}
	for i < len(p) {
		c := p[i]
		switch {
		case c == ']':
			b.WriteByte(']')
			return i + 1, true
		case c == '[' && i+1 < len(p) && strings.IndexByte(":.=", p[i+1]) >= 0:
			end := strings.Index(p[i+2:], string(p[i+1])+"]")
			if end < 0 {
				return 0, false
			}
			b.WriteString(p[i : i+2+end+2])
			i += 2 + end + 2
			continue
		case c == '\\' || c == '[' || c == '&':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	return 0, false
}
