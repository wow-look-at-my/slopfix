package english

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// A cut removes a phrase from its sentence. The parse decides what goes with
// it. A phrase sits in a clause through its punctuation. A comma comes before
// a phrase that ends the clause, and after one that opens it. A pair of commas
// goes round one in the middle. That punctuation belongs to the phrase and
// goes with it. The regular expression only finds the phrase.

// cutSpan answers the bytes of s to delete for the phrase the match covers.
// It is false when the parse forbids the cut: the match holds no word, or it
// takes part of a noun phrase. It leaves the rest without its head or its
// determiner.
func cutSpan(s string, from, to int) (int, int, bool) {
	sent := syntax.Parse(s, codeSpan.FindAllStringIndex(s, -1))
	first, last := -1, -1
	for i, w := range sent.Words {
		if w.Start >= from && w.End <= to && !isMark(w.Text) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || splitsNounPhrase(sent, first, last) {
		return 0, 0, false
	}

	prev, next := first-1, last+1
	before := prev >= 0 && sent.Words[prev].Text == ","
	after := next < len(sent.Words) && sent.Words[next].Text == ","
	closes := next >= len(sent.Words) || closer(sent.Words[next].Text)
	opens := prev < 0 || opener(sent.Words[prev].Text)

	// The match is the least the cut takes: a pattern can name the brackets round an aside.
	start := from + len(s[from:to]) - len(strings.TrimLeft(s[from:to], " \t\n"))
	end := from + len(strings.TrimRight(s[from:to], " \t\n"))
	switch {
	case before && after:
		// A phrase between a pair of commas takes both.
		start, end = sent.Words[prev].Start, sent.Words[next].End
	case before && closes:
		// A phrase that ends its clause takes the comma that opened it.
		start = min(start, sent.Words[prev].Start)
	case opens && after:
		// A phrase that opens its clause takes the comma that closed it.
		end = max(end, sent.Words[next].End)
	}
	return start, end, true
}

// join is the text that replaces a cut. The words on either side meet with
// one space, or with none before punctuation that closes something.
func join(s string, start, end int) (int, int, string) {
	for start > 0 && (s[start-1] == ' ' || s[start-1] == '\t') {
		start--
	}
	for end < len(s) && (s[end] == ' ' || s[end] == '\t') {
		end++
	}
	if start == 0 || end == len(s) || strings.ContainsRune(".,;:!?)]", rune(s[end])) || strings.ContainsRune("([", rune(s[start-1])) {
		return start, end, ""
	}
	return start, end, " "
}

// splitsNounPhrase reports a cut that takes some words of a noun phrase and
// leaves others. A cut of a whole noun phrase, or of no part of one, is fine.
func splitsNounPhrase(sent *syntax.Sentence, first, last int) bool {
	for _, p := range sent.Phrases {
		if p.Kind != syntax.NounPhrase {
			continue
		}
		overlaps := p.First <= last && first <= p.Last
		whole := first <= p.First && p.Last <= last
		if overlaps && !whole {
			return true
		}
	}
	return false
}

// isMark reports a token that is punctuation rather than a word.
func isMark(t string) bool {
	return strings.Trim(t, `.,;:!?()[]"'`+"`") == ""
}

// closer reports punctuation that ends a clause.
func closer(t string) bool {
	switch t {
	case ".", ";", ":", "!", "?", ")", "]", "...":
		return true
	}
	return false
}

// opener reports punctuation that starts a clause.
func opener(t string) bool {
	switch t {
	case ".", ";", ":", "!", "?", "(", "[", "\"":
		return true
	}
	return false
}
