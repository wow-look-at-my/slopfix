// rephrase.go applies the matcher language to a line of prose.
//
// An entry may write more words than it matched, fewer, or none at all, so a
// rule can swap a word, delete it, or rewrite the phrase around it. Prose no
// entry matches is left exactly as it is: the rule still reports the word, and
// a warning costs a reader less than a wrong repair.
package table

import "strings"

// A Rephrase is a match over word classes and what to write instead.
type Rephrase struct {
	ID string
	// Match is the source text, kept for the error a malformed entry reports
	// and for the name a test prints.
	Match string
	Terms Match
	To    string
	Where string
	Tests []Test
}

// A Normalize is a token rewrite applied before any match is tried, so a rule
// is written against a settled spelling rather than against all of them.
type Normalize struct {
	ID    string
	From  string
	To    string
	Tests []Test
}

// Rephrasings applies every entry to prose, earliest entry earliest.
func Rephrasings(lex *Lexicon, norms []Normalize, entries []Rephrase, prose string) string {
	spans := wordSpans(prose)
	if len(spans) == 0 || len(entries) == 0 {
		return prose
	}
	tokens := make([]string, len(spans))
	for i, s := range spans {
		tokens[i] = strings.ToLower(prose[s.at:s.end])
	}
	tokens = normalize(norms, tokens)

	var out strings.Builder
	last, i := 0, 0
	for i < len(tokens) {
		entry, end, caught, ok := firstMatch(lex, entries, tokens, i)
		if !ok {
			i++
			continue
		}
		out.WriteString(prose[last:spans[i].at])
		out.WriteString(Expand(entry.To, caught))
		last = spans[end-1].end
		i = end
	}
	out.WriteString(prose[last:])
	return closeGaps(out.String())
}

// firstMatch answers the earliest entry that fits at i.
func firstMatch(lex *Lexicon, entries []Rephrase, tokens []string, i int) (Rephrase, int, map[string]string, bool) {
	for _, entry := range entries {
		end, caught, ok := entry.Terms.find(lex, tokens, i)
		if ok && end > i {
			return entry, end, caught, true
		}
	}
	return Rephrase{}, 0, nil, false
}

// normalize rewrites a token wherever an entry names it. A rewrite carrying a
// space is refused: the repair writes back by word, so the counts must agree.
func normalize(norms []Normalize, tokens []string) []string {
	if len(norms) == 0 {
		return tokens
	}
	out := make([]string, len(tokens))
	copy(out, tokens)
	for _, n := range norms {
		if strings.ContainsAny(n.To, " \t") {
			continue
		}
		from := strings.ToLower(n.From)
		for i, tok := range out {
			if tok == from {
				out[i] = strings.ToLower(n.To)
			}
		}
	}
	return out
}

// closeGaps closes the doubled space a deletion leaves.
//
// It does NOT touch punctuation. The caller settles a dangling space before a
// full stop, and it reads what follows the stop before closing anything: a
// leading dot opens a name -- .github, .dats, .gitignore -- and a plain swap of
// " ." for "." ate the space in front of each of them.
func closeGaps(prose string) string {
	for strings.Contains(prose, "  ") {
		prose = strings.ReplaceAll(prose, "  ", " ")
	}
	return prose
}

// span is a word's place in the prose.
type span struct{ at, end int }

func wordSpans(prose string) []span {
	var out []span
	for i := 0; i < len(prose); {
		if !isWordByte(prose[i]) {
			i++
			continue
		}
		start := i
		for i < len(prose) && isWordByte(prose[i]) {
			i++
		}
		out = append(out, span{at: start, end: i})
	}
	return out
}
