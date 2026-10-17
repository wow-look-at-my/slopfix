package ste

// cutter finds where a window can end: never inside a span a division reads whole.
type cutter struct {
	source string
	// states holds the scan state at each position a window can start from.
	states map[int]scanState
}

// scanState is what a forward read holds open at a position.
type scanState struct {
	code, quote, curly, bold bool
	parens, brackets         int
}

func newCutter(source string) *cutter {
	return &cutter{source: source, states: map[int]scanState{0: {}}}
}

// neutral reports a state that holds nothing open.
func (s scanState) neutral() bool {
	return !s.code && !s.quote && !s.curly && !s.bold && s.parens == 0 && s.brackets == 0
}

// step moves the state over the byte at i.
func (s *scanState) step(text string, i int) {
	switch c := text[i]; {
	case c == '`':
		s.code = !s.code
	case s.code:
	case c == '"':
		s.quote = !s.quote
	case c == '\n':
		s.quote = false
	case c == '(':
		s.parens++
	case c == ')':
		s.parens = max(0, s.parens-1)
	case c == '[':
		s.brackets++
	case c == ']':
		s.brackets = max(0, s.brackets-1)
	case c == '*' && i+1 < len(text) && text[i+1] == '*' && (i == 0 || text[i-1] != '*'):
		s.bold = !s.bold
	case hasAt(text, i, "“"):
		s.curly = true
	case hasAt(text, i, "”"):
		s.curly = false
	}
}

func hasAt(text string, i int, mark string) bool {
	return len(text)-i >= len(mark) && text[i:i+len(mark)] == mark
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// longerThan reports a source of more than words words. It stops reading at
// the first word past the bound.
func (c *cutter) longerThan(words int) bool {
	n := 0
	for i := 0; i < len(c.source); i++ {
		if !isBlank(c.source[i]) && (i == 0 || isBlank(c.source[i-1])) {
			if n++; n > words {
				return true
			}
		}
	}
	return false
}

// after answers the length of the window that starts at from and holds at
// least words words, ending at a blank where the scan holds nothing open. It
// answers false when the rest is shorter, or has no such blank.
func (c *cutter) after(from, words int) (int, bool) {
	s, ok := c.states[from]
	if !ok {
		return 0, false
	}
	n := 0
	for i := from; i < len(c.source); i++ {
		if n >= words && isBlank(c.source[i]) && i > from && !isBlank(c.source[i-1]) && s.neutral() {
			for pos := range c.states {
				if pos < from {
					delete(c.states, pos)
				}
			}
			c.states[i] = s
			return i - from, true
		}
		if !isBlank(c.source[i]) && (i == from || isBlank(c.source[i-1])) {
			n++
		}
		s.step(c.source, i)
	}
	return 0, false
}
