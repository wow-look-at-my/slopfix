package ste

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
)

// Fix applies the repair Check names, for every rule Check reports.
func Fix(text string) string {
	return FixSelected(text, func(id string) bool { return !WarningIDs.Contains(id) })
}

// FixSelected applies the repairs whose ID keep accepts, so a caller that names
// a rule gets that rule's repair and no other.
func FixSelected(text string, keep func(id string) bool) string {
	return fixSelected(text, keep, true)
}

// FixKeepingOpening is Fix, with a division that keeps each sentence's
// opening words as its first sentence.
func FixKeepingOpening(text string) string {
	return fixSelected(text, func(id string) bool { return !WarningIDs.Contains(id) }, false)
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
		text = fixSentenceCap(text, capSpec{reorder: reorder, cap: SentenceWordCap, every: reorder})
	}
	return fixWarnings(text, keep, reorder)
}

// capSpec is how a long sentence divides. It holds the word cap each part
// must meet.
type capSpec struct {
	reorder bool
	cap     int
	// every divides a sentence no grammatical division reads, at a mark or between words.
	every bool
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
	locs := wordPattern.FindAllStringIndex(prose, -1)
	var out strings.Builder
	last := 0
	for _, loc := range locs {
		out.WriteString(prose[last:loc[0]])
		last = loc[1]
		word := prose[loc[0]:loc[1]]
		// A name such as should_hold contains a modal but is not one.
		if from, to := loc[0], loc[1]; from > 0 && isIdentByte(prose[from-1]) || to < len(prose) && isIdentByte(prose[to]) {
			out.WriteString(word)
			continue
		}
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
			out.WriteString(word)
			continue
		}
		if isCapitalized(word) {
			replacement = capitalize(replacement)
		}
		out.WriteString(replacement)
	}
	out.WriteString(prose[last:])
	return out.String()
}

func isIdentByte(b byte) bool { return b == '_' || b >= '0' && b <= '9' }

// inName reports whether the word at prose[from:to] is part of a longer name:
// an identifier, or a hyphenated name such as `ask-properly`. A dictionary
// repair inside a name renames the thing.
func inName(prose string, from, to int) bool {
	if from > 0 && isIdentByte(prose[from-1]) || to < len(prose) && isIdentByte(prose[to]) {
		return true
	}
	if from > 1 && prose[from-1] == '-' && isAlnum(prose[from-2]) {
		return true
	}
	return to+1 < len(prose) && prose[to] == '-' && isAlnum(prose[to+1])
}

func isAlnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// The comma is found the way checkSplices finds it, guard included, so the
// repair covers exactly what the check reports. A conjunction the connectors
// know gives way to its opener. Any other conjunction opens the new sentence.
// It reads a mask of the whole text, so a code span neither hides a splice nor
// cuts a sentence the parser needs whole.
func fixSplices(prose string) string {
	prose = fixAbsolutes(prose)
	masked := checkMask(prose)
	var joiners [][]int
	var openers []string
	parens := parenAsides(masked)
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

var (
	// absoluteParticiples close an absolute phrase: "the tests included".
	absoluteParticiples = set.Of(wordsOf("absolute-participle")...)
	// finiteVerbs are the verbs the splice pattern reads as a clause's own.
	finiteVerbs = set.Of(wordsOf("finite-verb")...)
	// passiveAuxiliaries make a participle after them the clause's verb.
	passiveAuxiliaries = set.Of("be", "been", "being", "get", "got", "gets")
	// absoluteAuxiliaries are the finite verbs that predicate the noun, so the words before the participle close a clause.
	absoluteAuxiliaries = set.Of(wordsOf("absolute-auxiliary")...)
	// clauseSubjects name the subject a relative clause puts before its verb.
	clauseSubjects = set.Of(wordsOf("clause-subject")...)
	// absolutePrepositions say an absolute phrase as a preposition before its noun.
	absolutePrepositions = map[string]string{"included": "including", "excluded": "excluding"}
)

// fixAbsolutes rewrites each splice the check reports whose words after the
// comma are an absolute phrase: a noun phrase closed by a participle. That
// fixAbsolutes is with no finite verb of the noun's own. "yours, the problems
// you did not cause included" names no clause, so a period there leaves a
// fragment. The phrase becomes "including the problems you did not cause", or
// takes "with" in front of a participle that has no preposition form.
func fixAbsolutes(prose string) string {
	masked := checkMask(prose)
	parens := parenthetical.FindAllStringIndex(masked, -1)
	spans := sentenceSpans(masked)
	type rewrite struct {
		from, to int
		text     string
	}
	var rewrites []rewrite
	for _, loc := range commaSplice.FindAllStringSubmatchIndex(masked, -1) {
		if loc[2] >= 0 || insideAny(parens, loc[0]) || !spliced(masked, loc) {
			continue
		}
		from := loc[0] + len(spliceComma.FindString(masked[loc[0]:]))
		to, last, ok := absoluteTail(masked, from, spans)
		if !ok {
			continue
		}
		participle := prose[last:to]
		phrase := strings.TrimRight(prose[from:last], " \t")
		text := "with " + phrase + " " + participle
		if preposition, known := absolutePrepositions[strings.ToLower(participle)]; known {
			text = preposition + " " + phrase
		}
		rewrites = append(rewrites, rewrite{from, to, text})
	}
	for i := len(rewrites) - 1; i >= 0; i-- {
		r := rewrites[i]
		prose = prose[:r.from] + r.text + prose[r.to:]
	}
	return prose
}

// absoluteTail answers the end of the absolute phrase that opens at byte. From
// and runs to the end of its sentence, and the start of its closing participle.
// The phrase holds no further mark. An auxiliary right before the participle
// predicates the noun: "the tests are included" is a clause. A finite verb
// there needs the subject a relative clause brings.
func absoluteTail(masked string, from int, spans [][2]int) (to, last int, ok bool) {
	end := -1
	for _, span := range spans {
		if span[0] <= from && from < span[1] {
			end = span[1]
		}
	}
	if end < 0 {
		return 0, 0, false
	}
	tail := strings.TrimRight(masked[from:end], " \t\n.!?")
	if strings.ContainsAny(tail, ",;:()[]\"") {
		return 0, 0, false
	}
	words := strings.Fields(tail)
	if len(words) < 2 || !absoluteParticiples.Contains(strings.ToLower(words[len(words)-1])) {
		return 0, 0, false
	}
	before := strings.ToLower(words[len(words)-2])
	if absoluteAuxiliaries.Contains(before) || passiveAuxiliaries.Contains(before) {
		return 0, 0, false
	}
	// A finite verb with a subject of its own sits in a relative clause
	// inside the noun phrase, as in "the logs it writes attached".
	if finiteVerbs.Contains(before) &&
		(len(words) < 4 || !clauseSubjects.Contains(strings.ToLower(words[len(words)-3]))) {
		return 0, 0, false
	}
	to = from + len(tail)
	return to, to - len(words[len(words)-1]), true
}

// offLimits are the spans no break may land in. The data Check hides, and a
// parenthetical, which STE counts as a single word and a break would halve.
func offLimits(prose, masked string) [][]int {
	off := verbatimSpan.FindAllStringIndex(prose, -1)
	return append(off, parenAsides(masked)...)
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
		if NameLike(prose[last:]) {
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
	if width == 0 || NameLike(s) {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}
