package ste

import (
	"regexp"
	"sort"
)

// formula matches a math expression in prose: terms joined by a spaced operator, with an optional "name =" in front.
var formula = regexp.MustCompile(`(?:\b[A-Za-z_]\w*\s*=\s*)?` + formulaTerm + `(?:\s+[-+*/=\x{00D7}]\s+(?:` + formulaTerm + `|[A-Za-z]\b))+`)

// spacedOperator is the operator between terms of a formula.
var spacedOperator = regexp.MustCompile(`\s[-+*/=\x{00D7}]\s`)

// formulaTerm is a term with a digit, a power or a fraction in it.
const formulaTerm = `[A-Za-z0-9_()]*[0-9^/][A-Za-z0-9_^/()]*(?:\.[0-9][A-Za-z0-9_^/()]*)*`

// formulaSpans answers each formula of prose that no other verbatim span holds.
func formulaSpans(prose string) [][]int {
	if !spacedOperator.MatchString(prose) {
		return nil
	}
	verbatim := verbatimSpan.FindAllStringIndex(prose, -1)
	var out [][]int
	for _, span := range formula.FindAllStringIndex(prose, -1) {
		if !overlapsAny(verbatim, span) {
			out = append(out, span)
		}
	}
	return out
}

// dataSpans answers every span of prose that is data rather than words: the
// verbatim spans and the formulas, in order.
func dataSpans(prose string) [][]int {
	out := append(verbatimSpan.FindAllStringIndex(prose, -1), formulaSpans(prose)...)
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// overlapsAny reports a span that shares a byte with one of spans.
func overlapsAny(spans [][]int, span []int) bool {
	for _, s := range spans {
		if span[0] < s[1] && s[0] < span[1] {
			return true
		}
	}
	return false
}

// stripFormulas writes CODE over each formula, as strip does over a code span.
func stripFormulas(text string) string {
	spans := formulaSpans(text)
	for i := len(spans) - 1; i >= 0; i-- {
		text = text[:spans[i][0]] + " CODE " + text[spans[i][1]:]
	}
	return text
}
