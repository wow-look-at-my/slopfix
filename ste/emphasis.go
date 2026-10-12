package ste

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// part is one piece of a rewritten clause.
type part struct {
	text     string
	from, to int
	verbatim bool
}

// joinParts writes the parts of a rewritten clause in order.
func joinParts(parts []part) string {
	var out strings.Builder
	for _, p := range parts {
		out.WriteString(p.text)
	}
	return out.String()
}

// emphasis is one span of bold or italic text: the delimiter. The offset of
// the delimiter that opens it and the offset of the one that closes it.
type emphasis struct {
	delim       string
	open, close int
}

// emphasisMarks answers each "**" and "__" span of the source outside code.
func emphasisMarks(source string) []emphasis {
	masked := mask(source)
	var out []emphasis
	for _, delim := range []string{"**", "__"} {
		open := -1
		for at := 0; at+len(delim) <= len(masked); {
			if masked[at:at+len(delim)] != delim {
				at++
				continue
			}
			after := at + len(delim)
			switch {
			case open < 0 && after < len(masked) && !isBlankByte(masked[after]) && (delim == "**" || at == 0 || !wordByte(masked[at-1])):
				open = at
			case open >= 0 && at > 0 && !isBlankByte(masked[at-1]) && (delim == "**" || after >= len(masked) || !wordByte(masked[after])):
				out = append(out, emphasis{delim, open, at})
				open = -1
			}
			at = after
		}
	}
	return out
}

// rewriteEmphasized writes the active voice for a passive clause that holds
// bold or italic text. The words are rewritten without the delimiters, and
// each delimiter pair then goes back around the words it marked, wherever
// the rewrite moved them.
func rewriteEmphasized(source string, marks []emphasis) (string, bool) {
	plain, spans := withoutDelimiters(source, marks)
	masked := mask(plain)
	parts, ok := passiveParts(syntax.Parse(masked, opaque(plain, masked)), plain)
	if !ok {
		return "", false
	}
	var out strings.Builder
	for _, p := range parts {
		out.WriteString(remark(p, spans))
	}
	text := out.String()
	for _, m := range marks {
		text = strings.ReplaceAll(text, m.delim+" "+m.delim, " ")
	}
	return text, true
}

// marked is the delimiter of an emphasis span and the plain bytes it marks.
type marked struct {
	delim    string
	from, to int
}

// withoutDelimiters answers the source with each delimiter of the marks cut,
// and the plain bytes each mark covers.
func withoutDelimiters(source string, marks []emphasis) (string, []marked) {
	type cut struct{ at, width int }
	var cuts []cut
	for _, m := range marks {
		cuts = append(cuts, cut{m.open, len(m.delim)}, cut{m.close, len(m.delim)})
	}
	sort.Slice(cuts, func(a, b int) bool { return cuts[a].at < cuts[b].at })
	shift := func(at int) int {
		n := at
		for _, c := range cuts {
			if c.at < at {
				n -= c.width
			}
		}
		return n
	}
	var plain strings.Builder
	last := 0
	for _, c := range cuts {
		plain.WriteString(source[last:c.at])
		last = c.at + c.width
	}
	plain.WriteString(source[last:])
	var spans []marked
	for _, m := range marks {
		spans = append(spans, marked{m.delim, shift(m.open + len(m.delim)), shift(m.close)})
	}
	return plain.String(), spans
}

// remark writes a part with the delimiters of each span that marks its bytes.
// A copied part takes them around the marked words alone. A part the rewrite
// wrote, such as a new verb, takes them whole.
func remark(p part, spans []marked) string {
	if p.from < 0 {
		return p.text
	}
	type insert struct {
		at    int
		delim string
	}
	var inserts []insert
	for _, sp := range spans {
		from, to := max(sp.from, p.from), min(sp.to, p.to)
		if from >= to {
			continue
		}
		if !p.verbatim || len(p.text) != p.to-p.from {
			inserts = append(inserts, insert{len(p.text), sp.delim}, insert{0, sp.delim})
			continue
		}
		x, y := from-p.from, to-p.from
		for x < y && isBlankByte(p.text[x]) {
			x++
		}
		for y > x && isBlankByte(p.text[y-1]) {
			y--
		}
		if x < y {
			inserts = append(inserts, insert{y, sp.delim}, insert{x, sp.delim})
		}
	}
	sort.SliceStable(inserts, func(a, b int) bool { return inserts[a].at > inserts[b].at })
	text := p.text
	for _, in := range inserts {
		text = text[:in.at] + in.delim + text[in.at:]
	}
	return text
}

func isBlankByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

// wordByte reports a byte that continues a word, for the intraword rule of "__".
func wordByte(c byte) bool {
	r, _ := utf8.DecodeRune([]byte{c})
	return c == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
