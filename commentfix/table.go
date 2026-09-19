// table.go applies the English the number repair writes instead of a number.
//
// The table itself is rules/, and nothing reads it at run time. cmd/rulegen
// parses that folder during the build. The generated file beside this holds
// every entry as a literal, with each pattern compiled to a switch automaton.
// Adding a phrase stays an edit to XML that needs no Go, and the binary carries
// no regexp engine.
package commentfix

import (
	"strings"
)

//go:generate go run github.com/wow-look-at-my/slopfix/cmd/rulegen -rules ../rules -for numbers -package commentfix -out numbers.gen.go

// Say rewrites a line of comment prose, applying every table entry. It says
// nothing about what is left: the caller checks that.
//
// The rewrites go before the patterns, so a shape reads the text a phrase swap
// has already settled.
func Say(prose string) string {
	original := prose
	for _, r := range numbersTable.Rewrites {
		prose = replaceWord(prose, r.From, r.To)
	}
	prose = sayOne(prose)
	for _, p := range numbersTable.Patterns {
		prose = p.Replace(prose)
	}
	prose = strings.Join(strings.Fields(prose), " ")
	prose = closeDanglingSpace(prose)
	return capitalise(original, prose)
}

// closeDanglingSpace removes the space a deletion leaves before punctuation
// that CLOSES something.
func closeDanglingSpace(prose string) string {
	var out strings.Builder
	for i := 0; i < len(prose); i++ {
		if !isSpaceByte(prose[i]) {
			out.WriteByte(prose[i])
			continue
		}
		next := i
		for next < len(prose) && isSpaceByte(prose[next]) {
			next++
		}
		if next < len(prose) && (prose[next] == '.' || prose[next] == ',') &&
			(next+1 >= len(prose) || isSpaceByte(prose[next+1])) {
			i = next - 1
			continue
		}
		out.WriteByte(prose[i])
	}
	return out.String()
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// replaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone.
func replaceWord(s, word, with string) string {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !wordBoundary(s, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(with)
		i = end
	}
	return b.String()
}

// wordBoundary reports whether s[at:end] stands as its own word. A marker
// joining it to a name spells a name that does not exist.
func wordBoundary(s string, at, end int) bool {
	if at > 0 && (isWordByte(s[at-1]) || joinsName(s, at-1, -1)) {
		return false
	}
	return end >= len(s) || !(isWordByte(s[end]) || joinsName(s, end, +1))
}

// nameMarkers join an identifier, an import path or a label into a name.
const nameMarkers = "._/:"

// joinsName reports whether s[at] is a marker with a name character beyond it,
// reading in the given direction.
func joinsName(s string, at, step int) bool {
	if strings.IndexByte(nameMarkers, s[at]) < 0 {
		return false
	}
	beyond := at + step
	return beyond >= 0 && beyond < len(s) && isWordByte(s[beyond])
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalise gives the repaired line the opening case the source line had. A
// comment line often continues a wrapped sentence rather than opening it, so
// the case the author wrote is the only reliable answer.
func capitalise(original, s string) string {
	if s == "" || original == "" {
		return s
	}
	upper := original[0] >= 'A' && original[0] <= 'Z'
	switch c := s[0]; {
	case upper && c >= 'a' && c <= 'z':
		return string(c-32) + s[1:]
	case !upper && c >= 'A' && c <= 'Z':
		return string(c+32) + s[1:]
	}
	return s
}
