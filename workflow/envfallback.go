package workflow

import "strings"

// envRead is one process.env.NAME read on a script line: the byte span the
// literal replaces, and whether the literal goes inside String().
type envRead struct {
	start, end int
	// wrap marks a read whose `??` arm runs past the line.
	wrap bool
}

// text answers what the literal writes over this read.
func (r envRead) text(literal string) string {
	if r.wrap {
		return "String(" + literal + ")"
	}
	return literal
}

// deadArms answers where the `??` arms after a read end. A string literal is
// never nullish, so each arm is dead. With no arm it answers from. An arm
// that does not end on this line answers from and true.
func deadArms(line string, from int) (int, bool) {
	at := from
	for {
		j := skipBlanks(line, at)
		if !strings.HasPrefix(line[j:], "??") || strings.HasPrefix(line[j:], "??=") {
			return at, false
		}
		end, ok := operandEnd(line, j+2)
		if !ok {
			return from, true
		}
		at = end
	}
}

// skipBlanks answers the first byte at or after i that is not a blank.
func skipBlanks(line string, i int) int {
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// operandEnd answers where the right operand of `??` that starts at i ends,
// after its last byte that is not a blank. The operand ends at a closing
// bracket, a mark or an operator that binds looser than `??`, at the same
// depth. The second result is false when the line ends first, because the
// operand can go on to the next line.
func operandEnd(line string, i int) (int, bool) {
	start, depth := i, 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == '"' || c == '\'' || c == '`':
			j := closeQuote(line, i)
			if j < 0 {
				return 0, false
			}
			i = j + 1
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case depth == 0 && (c == ')' || c == ']' || c == '}' || c == ';' || c == ',' || c == ':'):
			return trimmedEnd(line, start, i)
		case c == ')' || c == ']' || c == '}':
			depth--
		case depth == 0 && c == '?' && !optionalChain(line, i):
			return trimmedEnd(line, start, i)
		case depth == 0 && (strings.HasPrefix(line[i:], "||") || strings.HasPrefix(line[i:], "&&") || strings.HasPrefix(line[i:], "//")):
			return trimmedEnd(line, start, i)
		}
		i++
	}
	return 0, false
}

// optionalChain reports whether the `?` at i opens `?.`, which a digit after
// the dot makes a conditional instead.
func optionalChain(line string, i int) bool {
	if !strings.HasPrefix(line[i:], "?.") {
		return false
	}
	return i+2 >= len(line) || line[i+2] < '0' || line[i+2] > '9'
}

func closeQuote(line string, i int) int {
	q := line[i]
	for j := i + 1; j < len(line); j++ {
		if line[j] == '\\' {
			j++
			continue
		}
		if line[j] == q {
			return j
		}
	}
	return -1
}

// trimmedEnd answers the end of the operand from start to at, without its
// trailing blanks. An empty operand is no operand.
func trimmedEnd(line string, start, at int) (int, bool) {
	end := len(strings.TrimRight(line[:at], " \t"))
	if end <= start || strings.TrimSpace(line[start:end]) == "" {
		return 0, false
	}
	return end, true
}
