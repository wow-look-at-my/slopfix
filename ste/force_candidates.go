// force_candidates.go lists the word boundaries a forced division may cut at.

package ste

import (
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// phraseReach is how many words past the cap the phrase parse reads.
const phraseReach = 8

// phraseSpans answers the inside of each noun phrase and verb group the parser
// finds where a cut can land, and where each finite verb starts. A cut inside
// a phrase leaves "a detached." behind.
func phraseSpans(masked string, ends []int, limit int) ([][]int, set.Set[int], set.Set[int]) {
	finite, nounEnds := set.New[int](), set.New[int]()
	if len(ends) == 0 {
		return nil, finite, nounEnds
	}
	head := masked[:ends[min(limit+phraseReach, len(ends)-1)]]
	s := syntax.Parse(head, nil)
	var out [][]int
	for _, ph := range s.Phrases {
		if ph.First < ph.Last {
			// The span starts after the phrase's first byte, so the gap in front of it stays open.
			out = append(out, []int{s.Words[ph.First].Start + 1, s.Words[ph.Last].Start})
		}
	}
	for _, w := range s.Words {
		if w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "VBD" || w.Tag == "MD" {
			finite.Add(w.Start)
		}
		if strings.HasPrefix(w.Tag, "NN") {
			nounEnds.Add(w.End)
		}
	}
	return out, finite, nounEnds
}

// isWordByte reports a byte that can open bold text's first word: a letter,
// a digit or a code span.
func isWordByte(b byte) bool {
	return b == '`' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= 0x80
}

// wordEnds answers where each word WordCount counts ends, in order. A
// parenthetical is a single word that ends where it closes.
func wordEnds(masked string) []int {
	parens := parenthetical.FindAllStringIndex(masked, -1)
	var ends []int
	for _, span := range parens {
		ends = append(ends, span[1])
	}
	for _, loc := range wordPattern.FindAllStringIndex(masked, -1) {
		if !insideAny(parens, loc[0]) {
			ends = append(ends, loc[1])
		}
	}
	sort.Ints(ends)
	return ends
}

// candidates answers every admissible cut, best first. A strict pass also keeps
// each part off a word that leaves it hanging.
func candidates(source, masked string, strict bool, limit int) []forceCut {
	off := verbatimSpan.FindAllStringIndex(source, -1)
	off = append(off, quotedSpans(source)...)
	off = append(off, parenthetical.FindAllStringIndex(masked, -1)...)
	off = append(off, outerParens(masked)...)
	off = append(off, linkText.FindAllStringIndex(masked, -1)...)
	ends := wordEnds(masked)
	verbs := set.New[int]()
	spans, finite, nounEnds := phraseSpans(masked, ends, limit)
	if strict {
		off, verbs = append(off, spans...), finite
	}
	var out []forceCut
	counted := 0
	for _, gap := range gapRun.FindAllStringIndex(source, -1) {
		p, q := gap[0], gap[1]
		for counted < len(ends) && ends[counted] <= p {
			counted++
		}
		leftWords, rightWords := counted, len(ends)-counted
		if leftWords > limit {
			break
		}
		if p == 0 || q == len(source) || insideAny(off, p) || leftWords < 1 || rightWords < 1 {
			continue
		}
		if inBold(source, p) {
			continue
		}
		head := strings.TrimRight(source[:p], " ")
		// The part ends on the word the trim leaves, so a dash after "and" still ends it on "and".
		last := strings.ToLower(strings.Trim(lastField(strings.TrimRight(head, " ,;:—–-")), ".,;:!?*_\"'`()[]“”‘’"))
		next := strings.ToLower(strings.Trim(firstToken.FindString(source[q:]), ".,;:!?*_\"'`()[]“”‘’"))
		// A possessive governs the word after it, so a part never ends on one.
		possessive := strings.HasSuffix(last, "'s") || strings.HasSuffix(last, "’s")
		// A finite verb after the cut leaves its subject in the part before.
		if strict && (leftWords < minimumHalf || rightWords < minimumHalf || possessive || verbs.Contains(q) || forceDangling.Contains(last) || forceBound.Contains(next)) {
			continue
		}
		penalty := 8
		switch {
		case strings.ContainsAny(head[len(head)-1:], ",;:") || strings.HasSuffix(head, "—"):
			penalty = 0
		case forceOpener.Contains(next):
			penalty = 3
		}
		out = append(out, forceCut{left: p, right: q, score: leftWords - penalty, words: leftWords, afterNoun: nounEnds.Contains(p), bare: penalty == 8})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].score > out[b].score })
	return out
}

// outerParens answers each outermost balanced parenthesis, nested ones too. A
// link inside an aside nests a pair the parenthetical pattern cannot match.
func outerParens(text string) [][]int {
	var out [][]int
	depth, open := 0, 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			if depth == 0 {
				open = i
			}
			depth++
		case ')':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 {
				out = append(out, []int{open, i + 1})
			}
		}
	}
	return out
}
