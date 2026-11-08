// inventory_list_cut.go decides which part of a sentence an enumeration takes out.

package counts

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// dashes open or close an aside.
var dashes = []string{"—", "–", "--"}

// ListSpans answers the parts of one sentence that an enumeration takes out:
// a coordinated run of members, or a list behind a frame. A parenthesis is
// read as a list of its own.
//
// The cuts are tried in order: the aside that holds the run and the run with
// the preposition that introduces it. This also covers the clause after a
// colon that holds the run, and the run behind a lead-in word. The first cut
// whose remainder is still a statement wins. A sentence that was no
// statement, such as the label in a table cell, must keep a word. When no cut
// works, the span is the whole sentence: the list is its content.
func ListSpans(sentence string) ([]Span, bool) {
	whole := []Span{{0, len(sentence)}}
	masked := ste.Masked(sentence)
	end := len(strings.TrimRight(sentence, " \t\n.!?"))
	if end <= 0 {
		return nil, false
	}
	outer, outerMasked := []byte(sentence), []byte(masked)
	var asides [][]int
	for _, loc := range parenthetical.FindAllStringIndex(masked, -1) {
		asides = append(asides, loc)
		for i := loc[0]; i < loc[1]; i++ {
			outer[i], outerMasked[i] = ' ', ' '
		}
	}
	s := syntax.Parse(masked, inlineCode.FindAllStringIndex(sentence, -1))
	fragment := !statement(sentence)
	accept := func(spans []Span) bool {
		if fragment {
			return keepsWords(sentence, append([]Span(nil), spans...))
		}
		return statement(CutProse(sentence, append([]Span(nil), spans...)))
	}
	// A set noun and a complete-set word name the set: the list is the content.
	found, colon := framed(string(outer), string(outerMasked), end)
	if found && colon < 0 {
		return whole, true
	}
	if a, b, ok := coordinated(string(outer), string(outerMasked), s, 0, end); ok {
		for _, try := range runCuts(masked, s, a, b, end) {
			if accept(try) {
				return try, true
			}
		}
		return whole, true
	}
	for _, loc := range asides {
		if _, _, ok := coordinated(sentence, masked, s, loc[0]+1, loc[1]-1); ok {
			if try := []Span{{loc[0], loc[1]}}; accept(try) {
				return try, true
			}
			return whole, true
		}
	}
	if found {
		if try := []Span{{colon, end}}; accept(try) {
			return try, true
		}
		return whole, true
	}
	return nil, false
}

// runCuts answers each cut that can take out the run masked[a:b], in the
// order ListSpans tries them.
func runCuts(masked string, s *syntax.Sentence, a, b, end int) [][]Span {
	var out [][]Span
	aside, open := leadIn(masked, s, a, b, end)
	if aside != nil {
		out = append(out, aside)
	}
	if start, ok := preposition(s, a); ok {
		out = append(out, []Span{{start, b}})
	}
	if i := strings.LastIndex(masked[:a], ":"); i >= 0 {
		out = append(out, []Span{{i, end}})
	}
	if aside == nil && open >= 0 {
		out = append(out, []Span{{open, b}})
	}
	return out
}

// leadIn finds a lead-in word, such as "such as", right before the run
// masked[a:b]. aside is the cut when a comma or a dash opens the lead-in. It
// runs to the comma or the dash that closes the aside. An aside between a
// phrase with no finite verb and the main clause keeps one comma, so "To
// read files — such as `a`, `b`, or others — run" becomes "To read
// files, run".
func leadIn(masked string, s *syntax.Sentence, a, b, end int) (aside []Span, open int) {
	before := strings.TrimRight(masked[:a], " ")
	lower := strings.ToLower(before)
	open = -1
	for _, w := range listTable.WordsOf("lead-in") {
		phrase := strings.ReplaceAll(w, "_", " ")
		if strings.HasSuffix(lower, phrase) && (len(lower) == len(phrase) || !isLetter(lower[len(lower)-len(phrase)-1])) {
			open = len(before) - len(phrase)
			break
		}
	}
	if open < 0 {
		return nil, -1
	}
	pre := strings.TrimRight(masked[:open], " ")
	closer, from := "", open
	if strings.HasSuffix(pre, ",") {
		from, closer = len(pre)-1, ","
	}
	for _, d := range dashes {
		if strings.HasSuffix(pre, d) {
			from, closer = len(pre)-len(d), d
		}
	}
	if closer == "" {
		return nil, open
	}
	to := b
	if i := strings.Index(masked[b:end], closer); i >= 0 {
		to = b + i + len(closer)
	}
	if closer != "," && to < end && !finiteBefore(s, from) {
		if comma := strings.LastIndex(masked[b:to], ","); comma >= 0 {
			comma += b
			return []Span{{from, comma}, {comma + 1, to}}, open
		}
	}
	return []Span{{from, to}}, open
}

// finiteBefore reports a finite verb in a word that ends before at.
func finiteBefore(s *syntax.Sentence, at int) bool {
	for _, p := range s.Phrases {
		if p.Kind == syntax.VerbGroup && (p.Finite || p.Imperative) && s.Words[p.Last].End <= at {
			return true
		}
	}
	return false
}

// preposition answers where the phrase that introduces the run at a opens:
// a word of the run-preposition class. That preposition is after an
// optional conjunction and before an optional participle, as in "by
// setting `a`, `b` and `c`".
func preposition(s *syntax.Sentence, a int) (int, bool) {
	k := -1
	for i, w := range s.Words {
		if w.End <= a {
			k = i
		}
	}
	if k < 0 {
		return 0, false
	}
	if t := s.Words[k].Tag; t == "VBG" || t == "VBN" {
		k--
	}
	if k < 0 || !listTable.Lexicon().Is(s.Words[k].Lower(), "run-preposition") {
		return 0, false
	}
	if k > 0 && s.Words[k-1].Tag == "CC" {
		k--
	}
	return s.Words[k].Start, true
}

// statement reports text that holds a clause with a subject and a finite
// verb, or a bare verb that gives an instruction.
func statement(text string) bool {
	s := syntax.Parse(ste.Masked(text), inlineCode.FindAllStringIndex(text, -1))
	for _, c := range s.Clauses {
		if c.Subject != nil && c.Verb != nil && c.Verb.Finite {
			return true
		}
	}
	for _, p := range s.Phrases {
		if p.Kind == syntax.VerbGroup && p.Imperative {
			return true
		}
	}
	return false
}
