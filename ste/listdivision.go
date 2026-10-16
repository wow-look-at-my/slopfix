package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// dependentRest reports a cut whose rest cannot stand as a sentence of its own,
// though it holds a subject and a verb. One is a clause of manner: "as a
// variable such as q is". Another is the next item of a list whose verb came
// before the cut: "a digit, | an underscore or a dot inside it, or it is".
func dependentRest(whole *syntax.Sentence, c forceCut) bool {
	n := wordsBefore(whole, c.right)
	if n >= len(whole.Words) {
		return false
	}
	if mannerOnly(whole, n) {
		return true
	}
	if n < 1 || whole.Words[n-1].Text != "," || !nounOpener(whole.Words[n].Tag) {
		return false
	}
	// The rest continues the list when another comma or a conjunction comes before its first verb.
	for i := n + 1; i < len(whole.Words) && !finiteAt(whole, i); i++ {
		if whole.Words[i].Text == "," || whole.Words[i].Tag == "CC" {
			return true
		}
	}
	return false
}

// mannerOnly reports words from n on whose every finite verb sits in a clause
// of manner: "than the article, as a variable such as q is". Such words hold
// no main clause.
func mannerOnly(whole *syntax.Sentence, n int) bool {
	inside, finite := false, false
	for i := n; i < len(whole.Words); i++ {
		switch w := whole.Words[i]; {
		case w.Text == ",":
			inside = false
		case w.Lower() == "as" && finiteBefore(whole, i+1, ","):
			inside = true
		case finiteAt(whole, i) && !inside:
			return false
		case finiteAt(whole, i):
			finite = true
		}
	}
	return finite
}

// verbless reports words that hold no finite verb. A regular past form before
// a noun describes the noun, as in "merged opacity", and counts as none.
func verbless(s *syntax.Sentence) bool {
	for i, w := range s.Words {
		if !finiteAt(s, i) {
			continue
		}
		adjective := w.Tag == "VBD" && strings.HasSuffix(w.Lower(), "ed") && i+1 < len(s.Words) && strings.HasPrefix(s.Words[i+1].Tag, "NN")
		if !adjective {
			return false
		}
	}
	return true
}

// nounOpener reports a tag a noun phrase opens with.
func nounOpener(tag string) bool {
	return tag == "DT" || tag == "PRP$" || tag == "CD" || strings.HasPrefix(tag, "JJ") || strings.HasPrefix(tag, "NN")
}

// listDivision divides a list that holds no verb at all, such as the terms of a
// formula: "Per octant, the total mass, the mean, ..., and the flatness". It
// closes the list at the last comma that leaves the head under the cap, and
// the rest of the list stands behind "This also covers".
func listDivision(source, masked string, limit int) (string, bool) {
	if !verbless(syntax.Parse(masked, nil)) {
		return source, false
	}
	best := forceCut{words: -1}
	for _, c := range candidates(source, masked, false, limit) {
		if seamBefore(source, c.left) != "," || cutsAside(masked, c.left, c.right) || c.words < minimumHalf {
			continue
		}
		if WordCount(masked[c.right:]) >= minimumHalf && c.words > best.words {
			best = c
		}
	}
	if best.words < 0 {
		return source, false
	}
	rest := strings.TrimLeft(source[best.right:], " ")
	rest = rest[len(leadingConjunction.FindString(rest)):]
	return closeHead(source[:best.left]) + " This also covers " + rest, true
}
