package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// fragmentDivision divides a sentence that holds no main clause. Such a
// sentence is a noun phrase as written, so each part stays a noun phrase: the
// cut lands where a phrase ends and the next opens on a determiner. "a cache
// that never empties a cache that never fills" becomes "a cache that never
// empties. A cache that never fills".
func fragmentDivision(source, masked string, whole *syntax.Sentence, limit int) (string, bool) {
	if len(whole.Words) == 0 || standsAlone(whole, 0, len(whole.Words)) {
		return source, false
	}
	// Only a noun phrase is a fragment of its own. A sentence that opens on "to" or "if" leads into a clause the parser missed.
	switch t := whole.Words[0].Tag; {
	case t == "DT" || t == "JJ" || t == "PRP$" || strings.HasPrefix(t, "NN"):
	default:
		return source, false
	}
	for _, c := range candidates(source, masked, false, limit) {
		if seam := seamBefore(source, c.left); cutsAside(masked, c.left, c.right) || seam != "" && seam != "," {
			continue
		}
		n := wordsBefore(whole, c.left)
		if n < minimumHalf || len(whole.Words)-n < minimumHalf {
			continue
		}
		if !endsFragment(whole, n-1) || !opensFragment(whole.Words[n]) {
			continue
		}
		left := closeHead(source[:c.left])
		right := capitalizeOpening(strings.TrimLeft(source[c.right:], " "))
		if !divides(left, right, limit) {
			continue
		}
		return left + " " + right, true
	}
	return source, false
}

// opensFragment reports a word that opens a new noun phrase.
func opensFragment(w syntax.Word) bool {
	switch w.Lower() {
	case "a", "an", "the", "every", "each", "any", "some", "no":
		return true
	}
	return w.Tag == "PRP$"
}

// endsFragment reports a word i that can close a noun phrase or the clause
// inside one: a noun, an adverb, a number or a finite verb. A bare verb closes
// one only after a modal, as in "that will not fit". Otherwise it wants an object.
func endsFragment(s *syntax.Sentence, i int) bool {
	tag := s.Words[i].Tag
	if tag == "VB" {
		for j := i - 1; j >= max(0, i-2); j-- {
			if s.Words[j].Tag == "MD" {
				return true
			}
		}
		return false
	}
	return strings.HasPrefix(tag, "NN") || strings.HasPrefix(tag, "RB") || strings.HasPrefix(tag, "VB") || tag == "CD"
}
