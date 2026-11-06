// maxSet bounds a set, past which a search costs more than the regexp.

package precompiled

import (
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSet bounds a set, past which a search costs more than the regexp.
const maxSet = 64

// Required answers folded literals of which every match of pattern holds one.
// Nil means none are known. Text without them cannot match.
func Required(pattern string) []string {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	set, ok := required(re.Simplify())
	if !ok {
		return nil
	}
	slices.Sort(set)
	return minimal(slices.Compact(set))
}

// minimal drops each string that holds another string of the set. A text that
// holds the longer string holds the shorter, so the shorter is the whole test.
func minimal(set []string) []string {
	var out []string
	for _, s := range set {
		redundant := false
		for _, t := range set {
			if t != s && strings.Contains(s, t) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, s)
		}
	}
	return out
}

// required answers literals of which every match of re holds one.
func required(re *syntax.Regexp) ([]string, bool) {
	if set, ok := exact(re); ok && !slices.Contains(set, "") {
		return set, true
	}
	switch re.Op {
	case syntax.OpCapture, syntax.OpPlus:
		return required(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min >= 1 {
			return required(re.Sub[0])
		}
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			set, ok := required(sub)
			if !ok {
				return nil, false
			}
			all = append(all, set...)
		}
		return all, len(all) <= maxSet
	case syntax.OpConcat:
		return requiredConcat(re.Sub)
	}
	return nil, false
}

// requiredConcat picks the best set among the parts of a concatenation. A run
// of exact parts joins into the strings it spells, which are longer than any
// part alone.
func requiredConcat(subs []*syntax.Regexp) ([]string, bool) {
	var best []string
	found := false
	consider := func(set []string) {
		if len(set) == 0 || slices.Contains(set, "") {
			return
		}
		if !found || better(set, best) {
			best, found = set, true
		}
	}
	run := []string{""}
	inRun := false
	for _, sub := range subs {
		if set, ok := exact(sub); ok {
			if joined, ok := cross(run, set); ok {
				run, inRun = joined, true
				continue
			}
		}
		if inRun {
			consider(run)
		}
		run, inRun = []string{""}, false
		if set, ok := exact(sub); ok {
			run, inRun = set, true
			continue
		}
		if set, ok := required(sub); ok {
			consider(set)
		}
	}
	if inRun {
		consider(run)
	}
	return best, found
}

// better prefers the set whose shortest string is longer, then the smaller set.
func better(a, b []string) bool {
	la, lb := shortest(a), shortest(b)
	if la != lb {
		return la > lb
	}
	return len(a) < len(b)
}

func shortest(set []string) int {
	n := -1
	for _, s := range set {
		if n < 0 || len(s) < n {
			n = len(s)
		}
	}
	return n
}

// exact answers the whole set of strings that re matches, folded, when the
// set is small.
func exact(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return []string{""}, true
	case syntax.OpLiteral:
		return []string{Fold(string(re.Rune))}, true
	case syntax.OpCharClass:
		return classSet(re.Rune)
	case syntax.OpCapture:
		return exact(re.Sub[0])
	case syntax.OpQuest:
		set, ok := exact(re.Sub[0])
		if !ok {
			return nil, false
		}
		return append(set, ""), len(set)+1 <= maxSet
	case syntax.OpRepeat:
		if re.Max < 0 || re.Max > 4 {
			return nil, false
		}
		set, ok := exact(re.Sub[0])
		if !ok {
			return nil, false
		}
		var out []string
		for n := re.Min; n <= re.Max; n++ {
			times := []string{""}
			for range n {
				if times, ok = cross(times, set); !ok {
					return nil, false
				}
			}
			out = append(out, times...)
		}
		return out, len(out) <= maxSet
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			set, ok := exact(sub)
			if !ok {
				return nil, false
			}
			all = append(all, set...)
		}
		return all, len(all) <= maxSet
	case syntax.OpConcat:
		run := []string{""}
		for _, sub := range re.Sub {
			set, ok := exact(sub)
			if !ok {
				return nil, false
			}
			if run, ok = cross(run, set); !ok {
				return nil, false
			}
		}
		return run, true
	}
	return nil, false
}

// classSet answers the folded runes of a small character class.
func classSet(ranges []rune) ([]string, bool) {
	var out []string
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i+1]-ranges[i] >= maxSet {
			return nil, false
		}
		for r := ranges[i]; r <= ranges[i+1]; r++ {
			out = append(out, Fold(string(r)))
			if len(out) > maxSet {
				return nil, false
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out), len(out) > 0
}

// cross joins each string of a to each string of b.
func cross(a, b []string) ([]string, bool) {
	if len(a)*len(b) > maxSet {
		return nil, false
	}
	out := make([]string, 0, len(a)*len(b))
	for _, x := range a {
		for _, y := range b {
			out = append(out, x+y)
		}
	}
	return out, true
}

// Fold writes each rune of s as the least rune of its case-fold orbit.
// Strings that (?i) matches against each other fold to the same text. A
// literal that the text holds is held by the folded text too.
func Fold(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= utf8.RuneSelf || 'a' <= c && c <= 'z' {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteRune(foldRune(r))
	}
	return b.String()
}

// foldRune answers the least rune of r's case-fold orbit.
func foldRune(r rune) rune {
	if r < utf8.RuneSelf {
		if 'a' <= r && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}
	least := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		least = min(least, f)
	}
	return least
}

// HoldsAny reports whether folded, a text that Fold wrote, holds one of needs.
// Nil needs means that nothing is known, so the answer is true.
func HoldsAny(folded string, needs []string) bool {
	if needs == nil {
		return true
	}
	for _, n := range needs {
		if strings.Contains(folded, n) {
			return true
		}
	}
	return false
}
