// formula.go holds the formula code.

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
	verbatim := verbatimSpans(prose)
	var out [][]int
	for _, span := range formula.FindAllStringIndex(prose, -1) {
		if !overlapsAny(span, verbatim) {
			out = append(out, span)
		}
	}
	return out
}

// dataSpans answers every span of prose that is data rather than words: the
// verbatim spans and the formulas, in order.
func dataSpans(prose string) [][]int {
	out := append(verbatimSpans(prose), formulaSpans(prose)...)
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

