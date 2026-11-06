// relativein.go restates a clause a noun opens with "in which" as a sentence of its own.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// determiners open the subject of a clause rather than name its object.
var relativeDeterminers = set.Of("the", "a", "an", "this", "that", "these", "those", "it", "they", "he", "she", "we", "you")

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
		switch noun.Tag {
		case "DT", "IN", "CC", "TO", "PRP", "PRP$", "MD", "RB", "WDT", "WP", "WRB":
			continue
		}
		if indefinites.Contains(noun.Lower()) {
			continue
		}
		// A finite word among the first words after "which" that follows a
		// determiner belongs to a clause of its own, no list of members.
		if relativeDeterminers.Contains(s.Words[i+2].Lower()) && finiteBetween(s, i+2, min(i+5, len(s.Words))) {
			continue
		}
		// The word after "which" can be the filler the mask wrote over a code span.
		body := strings.TrimSpace(source[outsideSpans(source, s.Words[i+2].Start, false):])
		if body == "" || !holdsFinite(body) {
			continue
		}
		head := closeHead(source[:s.Words[i].Start])
		rest := "In that " + nounText(source, noun) + ", " + body
		if !divides(head, rest, limit) {
			continue
		}
		out := head + " " + closeHead(rest)
		if overCap(out, limit) < overCap(source, limit) {
			return out, true
		}
	}
	return source, false
}

// relativeThisDivision writes "..., which is a <noun>" as "This is <noun>":
// "..., which is a usage block that walks X" becomes "This is a usage block
// that walks X". The relative clause names the thing said again.
func relativeThisDivision(source string, s *syntax.Sentence, limit int) (string, bool) {
	for i := 1; i+2 < len(s.Words); i++ {
		if s.Words[i].Lower() != "which" || s.Words[i-1].Text != "," {
			continue
		}
		if s.Words[i+1].Lower() != "is" || s.Words[i+2].Lower() != "a" && s.Words[i+2].Lower() != "an" {
			continue
		}
		head := closeHead(source[:s.Words[i-1].Start])
		rest := "This " + source[s.Words[i+1].Start:]
		if !standsAsSentence(head) || !standsAsSentence(rest) || !divides(head, rest, limit) {
			continue
		}
		out := head + " " + closeHead(rest)
		if overCap(out, limit) < overCap(source, limit) {
			return out, true
		}
	}
	return source, false
}
