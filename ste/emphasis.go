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
	s := coverVerbatim(syntax.Parse(masked, opaque(plain, masked)), plain)
	parts, ok := passiveParts(s, plain)
	if !ok {
		return "", false
	}
	text := remarkParts(parts, spans)
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

// remarkParts writes the parts of a rewritten clause in order, with each
// delimiter pair around the run of parts its words landed in. A delimiter over
// one word marks that word alone, and one over the whole clause wraps the
// clause. A part the rewrite wrote, such as a new verb, takes the delimiters
// whole.
func remarkParts(parts []part, spans []marked) string {
	text := joinParts(parts)
	starts := make([]int, len(parts))
	at := 0
	for i, p := range parts {
		starts[i] = at
		at += len(p.text)
	}
	type insert struct {
		at    int
		delim string
	}
	var inserts []insert
	for _, sp := range spans {
		first, last := -1, -1
		for i, p := range parts {
			if p.from < 0 || max(sp.from, p.from) >= min(sp.to, p.to) {
				continue
			}
			if first < 0 {
				first = i
			}
			last = i
		}
		if first < 0 {
			continue
		}
		openAt, closeAt := starts[first]+partOffset(parts[first], sp.from, true), starts[last]+partOffset(parts[last], sp.to, false)
		if openAt >= closeAt {
			continue
		}
		inserts = append(inserts, insert{openAt, sp.delim}, insert{closeAt, sp.delim})
	}
	sort.SliceStable(inserts, func(a, b int) bool { return inserts[a].at > inserts[b].at })
	for _, in := range inserts {
		text = text[:in.at] + in.delim + text[in.at:]
	}
	return text
}

// partOffset answers the offset inside a part where a marked byte at opens or
// closes it. A part the rewrite wrote takes its whole text. A copied part
// lands on the marked words alone, past the blanks around them.
func partOffset(p part, at int, opening bool) int {
	if !p.verbatim || len(p.text) != p.to-p.from {
		if opening {
			return 0
		}
		return len(p.text)
	}
	if opening {
		x := max(at, p.from) - p.from
		for x < len(p.text) && isBlankByte(p.text[x]) {
			x++
		}
		return x
	}
	y := min(at, p.to) - p.from
	for y > 0 && isBlankByte(p.text[y-1]) {
		y--
	}
	return y
}

func isBlankByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

// wordByte reports a byte that continues a word, for the intraword rule of "__".
func wordByte(c byte) bool {
	r, _ := utf8.DecodeRune([]byte{c})
	return c == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
