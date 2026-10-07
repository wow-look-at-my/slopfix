package ste

import (
	"regexp"
	"sort"
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
func FixSelected(text string, keep func(id string) bool) string {
	return fixSelected(text, keep, true)
}

// FixKeepingOpening is Fix, with a division that keeps each sentence's
// opening words as its first sentence.
func FixKeepingOpening(text string) string {
	return fixSelected(text, func(string) bool { return true }, false)
}

// fixSelected runs the cap repair over the whole text rather than inside
// fixProse, because Check counts a code span as a single word.
func fixSelected(text string, keep func(id string) bool, reorder bool) string {
	text = fixProse(text, func(prose string) string { return fixWords(prose, keep) })
	if keep(IDSemicolon) {
		text = fixSemicolons(text)
	}
	if keep(IDCommaSplice) {
		text = fixSplices(text)
	}
	if keep(IDPostdeterminer) {
		text = fixPostdeterminers(text)
	}
	if keep(IDSentenceCap) {
		text = fixSentenceCap(text, capSpec{reorder: reorder, cap: SentenceWordCap})
	}
	return text
}

// capSpec is how a long sentence divides: the word cap each part must meet,
// and whether a division may move a clause or name. A subject apart.
type capSpec struct {
	reorder bool
	cap     int
}

// DivideTo divides each sentence of text over maxWords, as the cap repair
// does at its own cap, and keeps each sentence's opening words first.
func DivideTo(text string, maxWords int) string {
	return fixSentenceCap(text, capSpec{cap: maxWords})
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
// alone. A semicolon inside `a; b` is the thing the sentence documents. A
// quotation is another voice, so its words stay as Check reads them: unjudged.
func fixProse(text string, repair func(string) string) string {
	spans := append(verbatimSpan.FindAllStringIndex(text, -1), quotedSpans(text)...)
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	var out strings.Builder
	last := 0
	for _, span := range spans {
		if span[1] <= last {
			continue
		}
		if span[0] < last {
			span = []int{last, span[1]}
		}
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

// The comma is found the way checkSplices finds it, guard included, so the
// repair covers exactly what the check reports. A conjunction the connectors
// know gives way to its opener. Any other conjunction opens the new sentence.
// It reads a mask of the whole text, so a code span neither hides a splice nor
// cuts a sentence the parser needs whole.
func fixSplices(prose string) string {
	masked := checkMask(prose)
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

// offLimits are the spans no break may land in. The data Check hides, and a
// parenthetical, which STE counts as a single word and a break would halve.
func offLimits(prose, masked string) [][]int {
	off := verbatimSpan.FindAllStringIndex(prose, -1)
	return append(off, parenthetical.FindAllStringIndex(masked, -1)...)
}

// mask writes filler over every span strip hides from Check, byte for byte,
// so an offset into the mask is an offset into the prose. Both count the
// same words. A code span and an entity get the word strip writes.
func mask(prose string) string {
	out := []byte(prose)
	for _, span := range verbatimSpan.FindAllStringIndex(prose, -1) {
		switch prose[span[0]] {
		case '`':
			fillWith(out[span[0]:span[1]], "CODE")
		case '&':
			fillWith(out[span[0]:span[1]], "ENTITY")
		default:
			fillLink(out[span[0]:span[1]], fill)
		}
	}
	return string(out)
}

// fillLink fills a link target inside its brackets. Check reads "](URL)", so
// the parentheses stay and pair as Check pairs them.
func fillLink(span []byte, filler func([]byte)) {
	if len(span) < 3 || span[0] != ']' || span[1] != '(' || span[len(span)-1] != ')' {
		filler(span)
		return
	}
	filler(span[2 : len(span)-1])
}

// fillWith writes word over a span, with a space before it and blanks after it.
// A span too short to hold the word and its spaces gets the plain filler.
func fillWith(span []byte, word string) {
	if len(span) < len(word)+2 {
		fill(span)
		return
	}
	span[0] = ' '
	copy(span[1:], word)
	for i := 1 + len(word); i < len(span); i++ {
		span[i] = ' '
	}
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
			// No word follows to capitalize. A code span can still follow the text, so the space stays.
			joined := prose[joiner[0]:last]
			out.WriteString("." + joined[len(strings.TrimRight(joined, " \t")):])
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
