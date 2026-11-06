// relativein.go restates a clause a noun opens with "in which" as a sentence of its own.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// relativeInDivision writes "A block in which X share Y" as "A block. In that
// block, X share Y". The relative phrase names the noun again as the opening of
// the second sentence. A clause that opens on its own subject right after
// "which" is no list of members, so it stays with its noun.
func relativeInDivision(source string, s *syntax.Sentence, limit int) (string, bool) {
	for i := 2; i+2 < len(s.Words); i++ {
		if !strings.EqualFold(s.Words[i].Text, "in") || !strings.EqualFold(s.Words[i+1].Text, "which") {
			continue
		}
		noun := s.Words[i-1]
		if !strings.HasPrefix(noun.Tag, "NN") || indefinites.Contains(noun.Lower()) {
			continue
		}
		// Consider a finite verb among the first words after "which".
		if finiteBetween(s, i+2, min(i+5, len(s.Words))) {
			continue
		}
		body := strings.TrimSpace(source[s.Words[i+2].Start:])
		if body == "" || !holdsFinite(body) {
			continue
		}
		head := closeHead(source[:s.Words[i].Start])
		rest := "In that " + nounText(source, noun) + ", " + body
		if !standsAsSentence(rest) || !divides(head, rest, limit) {
			continue
		}
		out := head + " " + closeHead(rest)
		if overCap(out, limit) < overCap(source, limit) {
			return out, true
		}
	}
	return source, false
}
