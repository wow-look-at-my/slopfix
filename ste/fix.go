package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fix applies the repair Check names, for every rule.
func Fix(text string) string {
	return FixSelected(text, func(string) bool { return true })
}

// FixSelected applies the repairs whose ID keep accepts, so a caller that names
// a rule gets that rule's repair and no other.
//
// The cap repair runs over the whole text rather than inside fixProse. It
// measures a sentence the way Check measures it, and Check counts a code span
// as a single word rather than as a gap between shorter sentences.
func FixSelected(text string, keep func(id string) bool) string {
	text = fixProse(text, func(prose string) string {
		prose = fixWords(prose, keep)
		if keep(IDSemicolon) {
			prose = fixSemicolons(prose)
		}
		return prose
	})
	if keep(IDCommaSplice) {
		text = fixSplices(text)
	}
	if keep(IDPostdeterminer) {
		text = fixPostdeterminers(text)
	}
	if keep(IDSentenceCap) {
		text = fixSentenceCap(text)
	}
	return text
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
		replacement, banned := Expand(word)
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

func fixSemicolons(prose string) string {
	return breakWith(prose, semicolonRun.FindAllStringIndex(prose, -1), nil)
}

// The comma is found the way checkSplices finds it, guard included, so the
// repair covers exactly what the check reports. A conjunction the connectors
// know gives way to its opener. Any other conjunction opens the new sentence.
// It reads a mask of the whole text, so a code span neither hides a splice nor
// cuts a sentence the parser needs whole.
func fixSplices(prose string) string {
	masked := mask(prose)
	var joiners [][]int
	var openers []string
	parens := parenthetical.FindAllStringIndex(masked, -1)
	for _, loc := range commaSplice.FindAllStringSubmatchIndex(masked, -1) {
		if insideAny(parens, loc[0]) || !spliced(masked, loc) {
			continue
		}
		end := loc[0] + len(spliceComma.FindString(prose[loc[0]:]))
		opener := ""
		if loc[2] >= 0 {
			conjunction := strings.ToLower(strings.TrimSpace(prose[loc[2]:loc[3]]))
			if replaced, known := connectors[conjunction]; known {
				end, opener = loc[3], replaced
			}
		}
		joiners = append(joiners, []int{loc[0], end})
		openers = append(openers, opener)
	}
	return breakWith(prose, joiners, openers)
}

// offLimits are the spans no break may land in: the data Check hides, and a
// parenthetical, which STE counts as a single word and a break would halve.
func offLimits(prose, masked string) [][]int {
	off := verbatimSpan.FindAllStringIndex(prose, -1)
	return append(off, parenthetical.FindAllStringIndex(masked, -1)...)
}

// mask writes filler over every span strip hides from Check, byte for byte, so
// an offset into the mask is an offset into the prose and both count the same
// words.
func mask(prose string) string {
	out := []byte(prose)
	for _, span := range verbatimSpan.FindAllStringIndex(prose, -1) {
		fill(out[span[0]:span[1]])
	}
	return string(out)
}

// fill writes a single filler word over a span. It keeps a space at each end,
// so the words on either side stay words of their own.
func fill(span []byte) {
	for i := range span {
		span[i] = 'x'
	}
	if len(span) > 2 {
		span[0], span[len(span)-1] = ' ', ' '
	}
}

// breakWith rewrites each joiner span as a sentence break. A joiner's opener starts the
// new sentence when it has a single Otherwise the next word does, with a capital.
func breakWith(prose string, joiners [][]int, openers []string) string {
	var out strings.Builder
	last := 0
	for n, joiner := range joiners {
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
		out.WriteString(". ")
		if n < len(openers) && openers[n] != "" {
			out.WriteString(openers[n] + " ")
			continue
		}
		next, width := utf8.DecodeRuneInString(prose[last:])
		out.WriteRune(unicode.ToUpper(next))
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
