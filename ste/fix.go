package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fix applies the repair Check names. A long sentence is left alone,
// because splitting it needs a writer who knows the point.
func Fix(text string) string {
	return FixSelected(text, func(string) bool { return true })
}

// FixSelected applies the repairs whose ID keep accepts, so a caller that names
// a rule gets that rule's repair and no other.
func FixSelected(text string, keep func(id string) bool) string {
	return fixProse(text, func(prose string) string {
		prose = fixWords(prose, keep)
		if keep(IDSemicolon) {
			prose = fixSemicolons(prose)
		}
		if keep(IDCommaSplice) {
			prose = fixSplices(prose)
		}
		if keep(IDSentenceCap) {
			prose = fixSentenceCap(prose)
		}
		return prose
	})
}

var (
	// verbatimSpan matches the data strip also hides from Check. An entity ends
	// in a semicolon, which a repair must not touch.
	verbatimSpan = regexp.MustCompile(
		codeSpan.String() + `|` + linkTarget.String() + `|` + entity.String())
	// semicolonRun matches a semicolon and the space that follows it.
	semicolonRun = regexp.MustCompile(`;[ \t]*`)
	// spliceComma matches the comma checkSplices reports, and nothing past it.
	spliceComma = regexp.MustCompile(`,\s+`)
)

// fixProse applies repair to the prose of text, leaving each verbatim span
// alone. A semicolon inside `a; b` is the thing the sentence documents.
func fixProse(text string, repair func(string) string) string {
	var out strings.Builder
	last := 0
	for _, span := range verbatimSpan.FindAllStringIndex(text, -1) {
		out.WriteString(repair(text[last:span[0]]))
		out.WriteString(text[span[0]:span[1]])
		last = span[1]
	}
	out.WriteString(repair(text[last:]))
	return out.String()
}

// fixWords writes the approved word for each banned word, and keeps the
// capitalization the source used.
func fixWords(prose string, keep func(id string) bool) string {
	return wordPattern.ReplaceAllStringFunc(prose, func(word string) string {
		lower := strings.ToLower(word)
		replacement, banned := contractions[lower]
		if banned && !keep(IDContraction) {
			banned = false
		}
		if !banned {
			replacement, banned = modals[lower]
			if banned && !keep(IDModal) {
				banned = false
			}
		}
		if !banned {
			return word
		}
		if isCapitalized(word) {
			return capitalize(replacement)
		}
		return replacement
	})
}

// fixSemicolons writes the period the semicolon stands in for.
func fixSemicolons(prose string) string {
	return breakAt(prose, semicolonRun.FindAllStringIndex(prose, -1))
}

// fixSplices writes the period each spliced comma stands in for.
//
// The comma is found the way checkSplices finds it, guard included, so the
// repair covers exactly what the check reports. Only the comma is rewritten:
// a conjunction after it survives and opens the new sentence.
func fixSplices(prose string) string {
	var commas [][]int
	for _, loc := range commaSplice.FindAllStringSubmatchIndex(prose, -1) {
		if bare := loc[2] < 0; bare && !isClause(clauseBefore(prose, loc[0])) {
			continue
		}
		end := loc[0] + len(spliceComma.FindString(prose[loc[0]:]))
		commas = append(commas, []int{loc[0], end})
	}
	return breakAt(prose, commas)
}

// coordinator matches a conjunction joining clauses: the seam to divide at.
var coordinator = regexp.MustCompile(`,?\s+(?:and|but|so|then|because)\s+`)

// fixSentenceCap divides an over-cap sentence at the coordinator nearest its
// middle, repeating while a half is over. With none it is left for a writer.
func fixSentenceCap(prose string) string {
	for range maxDivisions {
		joiner, found := widestSeam(prose)
		if !found {
			return prose
		}
		prose = breakAt(prose, [][]int{joiner})
	}
	return prose
}

// maxDivisions bounds the repair: past this, no seam saves the sentence.
const maxDivisions = 8

// widestSeam answers the coordinator to divide at, inside the earliest sentence
// over the cap.
func widestSeam(prose string) ([]int, bool) {
	at := 0
	for _, sentence := range Sentences(prose) {
		start := strings.Index(prose[at:], strings.TrimSpace(sentence))
		if start < 0 {
			break
		}
		start += at
		end := start + len(strings.TrimSpace(sentence))
		at = end
		if WordCount(sentence) <= SentenceWordCap {
			continue
		}
		seams := coordinator.FindAllStringIndex(prose[start:end], -1)
		if len(seams) == 0 {
			continue
		}
		middle := (end - start) / 2
		best := seams[0]
		for _, seam := range seams {
			if abs(seam[0]-middle) < abs(best[0]-middle) {
				best = seam
			}
		}
		return []int{start + best[0], start + best[1]}, true
	}
	return nil, false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// breakAt rewrites each joiner span as a sentence break, and gives the word
// after it the capital a sentence opens with.
func breakAt(prose string, joiners [][]int) string {
	var out strings.Builder
	last := 0
	for _, joiner := range joiners {
		if joiner[0] < last {
			continue
		}
		out.WriteString(prose[last:joiner[0]])
		last = joiner[1]
		if last == len(prose) {
			// The joiner ends the text, so no word follows to open a sentence.
			out.WriteString(".")
			continue
		}
		next, width := utf8.DecodeRuneInString(prose[last:])
		out.WriteString(". " + string(unicode.ToUpper(next)))
		last += width
	}
	out.WriteString(prose[last:])
	return out.String()
}

func isCapitalized(word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	return unicode.IsUpper(first)
}

func capitalize(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if width == 0 {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}
