// rephrase.go applies the matcher language to a line of prose.
//
// An entry may write more words than it matched, fewer, or none at all. A
// rule can swap a word, delete it, or rewrite the phrase around it. Prose no
// entry matches is left exactly as it is: the rule still reports the word. A
// warning costs a reader less than a wrong repair.
package table

import "strings"

// A Rephrase is a match over word classes and what to write instead.
type Rephrase struct {
	ID string
	// Match is the source text, kept for the error a malformed entry reports and for the name a test prints.
	Match string
	Terms Match
	To    string
	Where string
	Tests []Test
}

// A Normalize settles a token's spelling before any match is tried.
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
	written := make([]string, len(spans))
	for i, s := range spans {
		written[i] = prose[s.at:s.end]
		tokens[i] = strings.ToLower(written[i])
	}
	tokens = normalize(norms, tokens)
	// A capture writes a word back, so it writes back the author's spelling.
	// A token the normalizer settled is that spelling now.
	for i, tok := range tokens {
		if tok != strings.ToLower(written[i]) {
			written[i] = tok
		}
	}

	var out strings.Builder
	last, i := 0, 0
	for i < len(tokens) {
		entry, end, caught, ok := firstMatch(lex, entries, tokens, i, written)
		if !ok || !spaced(prose, spans[i:end]) || hyphened(prose, spans[i], spans[end-1]) {
			i++
			continue
		}
		out.WriteString(prose[last:spans[i].at])
		matched := prose[spans[i].at:spans[end-1].end]
		out.WriteString(caseAt(prose, spans[i].at, matched, Expand(entry.To, caught)))
		last = spans[end-1].end
		i = end
	}
	out.WriteString(prose[last:])
	return closeGaps(out.String())
}

// caseAt is MatchCase for a match at offset at in prose. A capital opens the
// replacement only where the match opens a sentence, or where the author
// capitalized a plain word.
func caseAt(prose string, at int, matched, replacement string) string {
	if !opensSentence(prose[:at]) && emphatic(prose[:at], matched) {
		return replacement
	}
	return MatchCase(matched, replacement)
}

// opensSentence reports whether the text before a word ends a sentence, or is empty.
func opensSentence(before string) bool {
	trimmed := strings.TrimRight(before, " \t\r\n")
	return trimmed == "" || strings.ContainsAny(trimmed[len(trimmed)-1:], ".!?")
}

// emphatic reports a first word in capitals, or a capitalized first word after
// a lower-case word. Either way, the capital is not a sentence opener.
func emphatic(before, matched string) bool {
	word := matched
	if end := strings.IndexFunc(matched, func(r rune) bool { return r < 'A' || (r > 'Z' && r < 'a') || r > 'z' }); end >= 0 {
		word = matched[:end]
	}
	if len(word) > 1 && strings.ToUpper(word) == word && strings.ToLower(word) != word {
		return true
	}
	prev := strings.Fields(before)
	if len(prev) == 0 {
		return false
	}
	last := prev[len(prev)-1]
	return last[0] >= 'a' && last[0] <= 'z'
}

// spaced reports whether whitespace alone joins the words, so a hyphenated compound stays whole.
func spaced(prose string, words []span) bool {
	for i := 1; i < len(words); i++ {
		if strings.TrimSpace(prose[words[i-1].end:words[i].at]) != "" {
			return false
		}
	}
	return true
}

// hyphened reports a match whose edge word a hyphen joins to the word beside
// it. "the two" in "the two-line row" is part of a compound.
func hyphened(prose string, first, last span) bool {
	before := first.at >= 2 && prose[first.at-1] == '-' && isWordByte(prose[first.at-2])
	after := last.end+1 < len(prose) && prose[last.end] == '-' && isWordByte(prose[last.end+1])
	return before || after
}

// firstMatch answers the earliest entry that fits at i.
func firstMatch(lex *Lexicon, entries []Rephrase, tokens []string, i int, written []string) (Rephrase, int, map[string]string, bool) {
	for _, entry := range entries {
		end, caught, ok := entry.Terms.find(lex, tokens, written, i)
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
func closeGaps(prose string) string {
	for strings.Contains(prose, "  ") {
		prose = strings.ReplaceAll(prose, "  ", " ")
	}
	return prose
}

// isWordByte reports the character class a word boundary reads.
func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
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
