package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
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
		if keep(IDCommaSplice) {
			prose = fixSplices(prose)
		}
		return prose
	})
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
	return breakAt(prose, semicolonRun.FindAllStringIndex(prose, -1))
}

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

// The seams a division can take, strongest earliest. Each is a place the
// sentence already divides in the reader's head: a conjunction, or the comma
// before one. A sentence carrying none of them is left at its length.
//
// The coordinator puts the conjunction inside the group, because the clause
// after it already opens a sentence. Every weaker seam leaves the conjunction
// standing, so no word is lost to a repair nobody reviews.
var (
	// coordinator matches a conjunction joining clauses: a candidate seam.
	coordinator = regexp.MustCompile(`(,?\s+(?:and|but|so|then|because)\s+)`)
	// clauseSeam matches the comma a writer put between clauses.
	clauseSeam = regexp.MustCompile(`(,\s+)(?:and|but|so|or|yet|then|because|since|which|` +
		`while|although|though|unless|after|before|until|whenever|when|where|if|` +
		`rather|instead|except)\s`)
	// bareSeam matches the same conjunctions carrying no comma.
	bareSeam = regexp.MustCompile(`(\s+)(?:and|but|so|or|yet|then|because|since|which|while)\s`)
)

// opensASubject holds the words an independent clause starts its subject with.
// A coordinator followed by any of them joins clauses that each name who acts.
// A coordinator followed by anything else joins verbs that SHARE a subject,
// where a division writes a sentence with nobody in it.
var opensASubject = set.Of("i", "we", "you", "he", "she", "it", "they", "one",
	"this", "that", "these", "those", "there", "here",
	"the", "a", "an", "every", "each", "any", "no", "some", "all", "both",
	"either", "neither", "another", "such",
	"its", "their", "his", "her", "our", "your", "my")

// carriesItsOwnSubject reports whether the clause after a seam names who acts.
// A capital opens a name, which is a subject of its own.
func carriesItsOwnSubject(clause string) bool {
	word, _, _ := strings.Cut(strings.TrimSpace(clause), " ")
	word = strings.Trim(word, `"'`+"`([")
	if word == "" {
		return false
	}
	if first, _ := utf8.DecodeRuneInString(word); unicode.IsUpper(first) {
		return true
	}
	return opensASubject.Contains(strings.ToLower(word))
}

// fixSentenceCap divides every over-cap sentence.
func fixSentenceCap(prose string) string {
	// A division always shortens the sentence it cuts.
	for range len(strings.Fields(prose)) + 1 {
		joiner, found := nextDivision(prose)
		if !found {
			return prose
		}
		prose = breakAt(prose, [][]int{joiner})
	}
	return prose
}

// nextDivision answers the span to rewrite as a break, inside the earliest
// sentence over the cap.
func nextDivision(prose string) ([]int, bool) {
	masked := mask(prose)
	off := offLimits(prose, masked)
	at := 0
	for _, sentence := range Sentences(masked) {
		start := strings.Index(masked[at:], sentence)
		if start < 0 {
			break
		}
		start += at
		end := start + len(sentence)
		at = end
		if WordCount(sentence) <= SentenceWordCap {
			continue
		}
		if cut, ok := divide(masked, off, start, end); ok {
			return cut, true
		}
	}
	return nil, false
}

// divide picks where to cut a sentence, taking the best seam kind that has a
// usable place in it.
//
// A sentence offering no such place keeps its length and is reported: a cut
// anywhere else writes a fragment, which is a worse document than a long
// sentence. Cutting at any space wrote "the ID names an. Existing entry" into a
// checked-in file.
func divide(masked string, off [][]int, start, end int) ([]int, bool) {
	for _, seam := range []*regexp.Regexp{coordinator, clauseSeam, bareSeam} {
		if cut, ok := nearestMiddle(masked, off, start, end, seam, carriesItsOwnSubject); ok {
			if serialList(masked[start:cut[0]]) {
				continue
			}
			return cut, true
		}
	}
	return nil, false
}

// serialList reports whether the half before a seam is a list of items. The
// last "and" of "the helpers, the backends and the daemon" joins nouns
// rather than clauses, and a cut there leaves the verb behind.
func serialList(head string) bool {
	return strings.Contains(head, ",")
}

// nearestMiddle answers the usable seam closest to the sentence's middle, which
// is where a division leaves both halves most alike.
func nearestMiddle(masked string, off [][]int, start, end int, seam *regexp.Regexp, wants func(string) bool) ([]int, bool) {
	middle := (end - start) / 2
	var pick []int
	for _, at := range seam.FindAllStringSubmatchIndex(masked[start:end], -1) {
		cut := []int{start + at[2], start + at[3]}
		if !usable(masked, off, cut, start, end) {
			continue
		}
		if wants != nil && !wants(masked[cut[1]:end]) {
			continue
		}
		if pick == nil || abs(at[2]-middle) < abs(pick[0]-start-middle) {
			pick = cut
		}
	}
	return pick, pick != nil
}

// usable reports whether a seam is a place a sentence break can go.
func usable(masked string, off [][]int, cut []int, start, end int) bool {
	if cut[0] <= start || cut[1] >= end {
		return false // a half with no words in it is not a sentence
	}
	for _, span := range off {
		if cut[0] < span[1] && span[0] < cut[1] {
			return false // inside a code span, a link, an entity or a parenthetical
		}
	}
	if last, _ := utf8.DecodeLastRuneInString(masked[:cut[0]]); terminator(last) {
		return false // the sentence already ends here, and another period reads as an ellipsis
	}
	if WordCount(masked[start:cut[0]]) == 0 || WordCount(masked[cut[1]:end]) == 0 {
		return false
	}
	// breakAt capitalizes what follows, so the new sentence has to open with something a capital applies to. Sentences
	first, _ := utf8.DecodeRuneInString(masked[cut[1]:])
	return unicode.IsLetter(first) || unicode.IsDigit(first)
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// breakAt rewrites each joiner span as a sentence break.
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
