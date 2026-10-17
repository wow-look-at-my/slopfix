package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
)

// wrap lays prose back onto comment lines at the width the paragraph had. A
// word longer than the width goes on its own line rather than being broken. A
// URL or an identifier split across lines stops being either.
func wrap(prose, marker, cont string, width int) []string {
	words := strings.Fields(prose)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line, at := marker, marker
	var last string
	count := 0
	for _, w := range words {
		if line != at && len(line)+len(w) > width {
			// A number never opens a line: the word before it carries down, so "exit" stays with "2".
			carry := ""
			if count > 1 && opensWithDigit(w) {
				carry = last
				line = strings.TrimSuffix(line, carry+" ")
			}
			out = append(out, strings.TrimRight(line, " "))
			at = cont
			line = cont
			count = 0
			if carry != "" {
				line += carry + " "
				count = 1
			}
		}
		line += w + " "
		last = w
		count++
	}
	return append(out, strings.TrimRight(line, " "))
}

// opensWithDigit reports a word that starts on a digit.
func opensWithDigit(w string) bool {
	return w != "" && w[0] >= '0' && w[0] <= '9'
}

// numberLeadsLine reports a paragraph with a line after its first that opens
// on a number. That number is a finding of its own line. Wrapping the
// paragraph again moves the word before the number down beside it.
func numberLeadsLine(lines []string, p para) bool {
	for _, i := range p.lines[min(1, len(p.lines)):] {
		_, prose, _, ok := splitBlock(lines[i])
		if !ok {
			continue
		}
		for _, found := range cardinal.Find(prose, cardinal.Comment) {
			if found.Offset == 0 {
				return true
			}
		}
	}
	return false
}
