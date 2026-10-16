package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// markfallback.go divides a long sentence that every grammatical division
// refused. A dash aside and a colon divide first, because each already marks
// where one thought ends. Any word boundary serves last, so the cap rule
// leaves no finding standing.

// asideDivision divides a sentence at a pair of dashes. The words after the
// closing dash open a sentence of their own, past the conjunction that joined
// them. The aside stays with the words before it behind a comma. An aside that
// stands as a sentence becomes a sentence of its own instead:
// "X -- having paid Y -- so Z." becomes "X, having paid Y. Z."
func asideDivision(source, masked string, limit int) (string, bool) {
	dashes := asideDash.FindAllStringIndex(masked, -1)
	parens := outerParens(masked)
	best, bestOver := "", overCap(source, limit)
	for i := 0; i+1 < len(dashes); i += 2 {
		open, closing := dashes[i], dashes[i+1]
		if insideAny(parens, open[0]) || insideAny(parens, closing[0]) || inBold(source, open[0]) {
			continue
		}
		before := strings.TrimRight(source[:open[0]], " ,")
		body := strings.TrimSpace(source[open[1]:closing[0]])
		after := strings.TrimSpace(source[closing[1]:])
		if WordCount(checkMask(before)) < minimumHalf || body == "" {
			continue
		}
		rest, restOK := restAfterMark(after)
		var tries []string
		if restOK {
			tries = append(tries, closeHead(before+", "+body)+" "+rest)
		}
		if aside := capitalizeOpening(body); !lowerIdentifier(firstToken.FindString(body)) && StandsAlone(aside) {
			out := closeHead(before) + " " + closeHead(aside)
			switch {
			case restOK:
				out += " " + rest
			case strings.Trim(after, ".!? ") != "":
				// The words after the aside read as no sentence, so the aside keeps its place.
				continue
			}
			tries = append(tries, out)
		}
		for _, out := range tries {
			if over := overCap(out, limit); over < bestOver && splitsAsWritten(out, before) {
				best, bestOver = out, over
			}
		}
	}
	return best, best != ""
}

// restAfterMark writes the words after a colon or a dash as a sentence of their
// own. A conjunction that opened them gives way to its opener, as at a comma.
// It answers false when the words hold no main clause.
func restAfterMark(after string) (string, bool) {
	rest := strings.TrimLeft(after, " —–-,;:")
	if strings.Trim(rest, ".!? ") == "" {
		return "", false
	}
	opener := ""
	if word := firstToken.FindString(rest); word != "" {
		if connector, ok := connectors[strings.ToLower(word)]; ok {
			opener = connector
			rest = strings.TrimLeft(rest[len(word):], " —–-,;:")
		}
	}
	if rest == "" || lowerIdentifier(firstToken.FindString(rest)) {
		return "", false
	}
	out := joinOpener(opener, rest)
	if !StandsAlone(out) {
		return "", false
	}
	return out, true
}

// standingMarkDivision divides a sentence at a colon or a lone dash. The words
// after the mark must hold a main clause. The words before it close as a
// sentence however they read, because the mark already ends a thought there.
func standingMarkDivision(source, masked string, limit int) (string, bool) {
	best, bestOver, bestWords := "", overCap(source, limit), -1
	for _, c := range candidates(source, masked, false, limit) {
		seam := seamBefore(source, c.left)
		if seam != ":" && !seamIsDash(seam) || cutsAside(masked, c.left, c.right) {
			continue
		}
		if c.words < minimumHalf {
			continue
		}
		rest, ok := restAfterMark(source[c.right:])
		if !ok {
			continue
		}
		left := closeHead(source[:c.left])
		if !divides(left, rest, limit) {
			continue
		}
		out := left + " " + rest
		over := overCap(out, limit)
		if over < bestOver || over == bestOver && best != "" && c.words > bestWords {
			best, bestOver, bestWords = out, over, c.words
		}
	}
	return best, best != ""
}

// anyDivision is the last division. It cuts at the word boundary that reads
// best among those that leave the first part under the cap: a part. That does
// not end on a word that needs the next one, then a cut at a mark or before a
// conjunction. Then the longest first part. The cap rule then has a repair for
// every sentence over it.
func anyDivision(source, masked string, limit int) (string, bool) {
	var parts []string
	rest, restMasked := source, masked
	for len(wordEnds(restMasked)) > limit {
		left, right, ok := anyCut(rest, restMasked, limit)
		if !ok {
			break
		}
		parts = append(parts, left)
		rest, restMasked = right, checkMask(right)
	}
	if len(parts) == 0 {
		return source, false
	}
	return strings.Join(append(parts, rest), " "), true
}

// anyCut answers the first part anyDivision cuts from source, closed, and the rest, opened.
func anyCut(source, masked string, limit int) (string, string, bool) {
	ends := wordEnds(masked)
	// Only the words near the cap decide a cut, so only they are parsed.
	whole := syntax.Parse(masked[:ends[min(limit+phraseReach, len(ends)-1)]], nil)
	best, bestRest, bestKey := "", "", [3]int{-1, -1, -1}
	for _, keepAsides := range []bool{true, false} {
		for _, c := range candidates(source, masked, false, limit) {
			if keepAsides && cutsAside(masked, c.left, c.right) {
				continue
			}
			head := strings.TrimRight(source[:c.left], " ,;:—–-")
			restText := strings.TrimLeft(source[c.right:], " —–-,;:")
			next := firstToken.FindString(restText)
			if head == "" || next == "" || lowerIdentifier(next) {
				continue
			}
			left, right := closeHead(head), capitalizeOpening(restText)
			if !divides(left, right, limit) {
				continue
			}
			last := strings.ToLower(strings.Trim(lastField(head), ".,;:!?*_\"'`()[]“”‘’"))
			bare := strings.ToLower(strings.Trim(next, ".,;:!?*_\"'`()[]“”‘’"))
			loose := 1
			if forceDangling.Contains(last) || forceBound.Contains(bare) || strings.HasSuffix(last, "'s") || strings.HasSuffix(last, "’s") {
				loose = 0
			}
			key := [3]int{loose, hardSeamRank(source, whole, c), c.words}
			if key[0] > bestKey[0] || key[0] == bestKey[0] && (key[1] > bestKey[1] || key[1] == bestKey[1] && key[2] > bestKey[2]) {
				best, bestRest, bestKey = left, right, key
			}
		}
		if best != "" {
			return best, bestRest, true
		}
	}
	return "", "", false
}

// splitsAsWritten reports whether Check reads the division as written: the
// first sentence of out is the words before the mark, closed.
func splitsAsWritten(out, before string) bool {
	sentences := Sentences(checkMask(out))
	return len(sentences) >= 2 && WordCount(sentences[0]) >= WordCount(checkMask(before))
}
