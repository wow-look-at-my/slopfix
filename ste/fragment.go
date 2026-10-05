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
	for _, c := range candidates(source, masked, false, limit) {
		if seam := seamBefore(source, c.left); cutsAside(masked, c.left, c.right) || seam != "" && seam != "," {
			continue
		}
		n := wordsBefore(whole, c.left)
		if n < minimumHalf || len(whole.Words)-n < minimumHalf {
			continue
		}
		last, next := whole.Words[n-1], whole.Words[n]
		if !endsFragment(last.Tag) || !opensFragment(next) {
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

// endsFragment reports a tag that can close a noun phrase or the clause inside
// one: a noun, an adverb, a verb or a number.
func endsFragment(tag string) bool {
	return strings.HasPrefix(tag, "NN") || strings.HasPrefix(tag, "RB") || strings.HasPrefix(tag, "VB") || tag == "CD"
}
