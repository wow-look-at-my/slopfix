package ste

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// force.go divides a long sentence that has no clause boundary. The division
// lands between words, near the cap, where each part is a sentence.

var (
	// forceDangling words never end the first part of a forced division.
	forceDangling = set.Of(wordsOf("force-dangling")...)
	// forceBound words never open the second part.
	forceBound = set.Of(wordsOf("force-bound")...)
	// forceOpener words open a phrase, so a division in front of one reads better.
	forceOpener = set.Of(wordsOf("force-opener")...)
	// linkText is the bracketed text of a markdown link, which no division halves.
	linkText = regexp.MustCompile(`\[[^\]]*\]`)
	// gapRun is the blank between a couple of words.
	gapRun = regexp.MustCompile(`\s+`)
	// firstToken is the word that opens a part.
	firstToken = regexp.MustCompile(`^\S+`)
)

// forceSentenceCap divides each sentence still over the cap at a word boundary.
func forceSentenceCap(prose string) string {
	over := overCap(prose)
	for range len(strings.Fields(prose)) + 1 {
		next, divided := forceNext(prose)
		// A division that leaves as many words past the cap moves nothing, so the loop stops on it.
		if !divided || overCap(next) >= over {
			return prose
		}
		prose, over = next, overCap(next)
	}
	return prose
}

// overCap counts the words past the cap in every sentence Check reads in prose.
func overCap(prose string) int {
	n := 0
	for _, sentence := range Sentences(checkMask(prose)) {
		n += max(0, WordCount(sentence)-SentenceWordCap)
	}
	return n
}

// forceNext divides the earliest over-cap sentence, as Check reads it.
func forceNext(prose string) (string, bool) {
	masked := checkMask(prose)
	for _, span := range sentenceSpans(masked) {
		start, end := span[0], span[1]
		// A filled span ends in a blank where the prose has its mark, so the sentence takes the mark back.
		for start > 0 && masked[start-1] == ' ' && prose[start-1] != ' ' {
			start--
		}
		for end < len(prose) && masked[end] == ' ' && prose[end] != ' ' {
			end++
		}
		if WordCount(masked[start:end]) <= SentenceWordCap {
			continue
		}
		if rewritten, ok := forceDivision(prose[start:end], masked[start:end]); ok {
			return prose[:start] + rewritten + prose[end:], true
		}
	}
	return prose, false
}

// Masked is the prose as Check reads it, with every offset kept.
func Masked(prose string) string { return checkMask(prose) }

// checkMask writes the word Check's strip writes over each span it strips,
// byte for byte. The sentences, the word counts and the tags then match Check,
// and an offset into the mask is an offset into the prose.
func checkMask(prose string) string {
	out := []byte(prose)
	for _, span := range verbatimSpan.FindAllStringIndex(prose, -1) {
		switch prose[span[0]] {
		case '`':
			fillCheckWord(out[span[0]:span[1]], "CODE")
		case '&':
			fillCheckWord(out[span[0]:span[1]], "ENTITY")
		default:
			fillLink(out[span[0]:span[1]], func(inner []byte) { fillCheckWord(inner, "URL") })
		}
	}
	// Check finds a quotation after it strips code, so a quote mark inside a code span pairs with nothing.
	for _, span := range quotation.FindAllIndex(out, -1) {
		fillCheckWord(out[span[0]:span[1]], "QUOTE")
	}
	return string(out)
}

// fillCheckWord writes word over a span as fillWith does. A span too short to
// hold it gets the capitalized filler.
func fillCheckWord(span []byte, word string) {
	if len(span) < len(word)+2 {
		fillWord(span)
		return
	}
	fillWith(span, word)
}

// quotedSpans answers the quotations Check reads in prose, after code is masked.
func quotedSpans(prose string) [][]int {
	return quotation.FindAllStringIndex(mask(prose), -1)
}

// fillWord writes a capitalized filler word over a span, as Check writes CODE
// or QUOTE. A space at each end keeps the words beside it apart.
func fillWord(span []byte) {
	for i := range span {
		span[i] = 'X'
	}
	if len(span) > 2 {
		span[0], span[len(span)-1] = ' ', ' '
	}
}

// forceCut is a place between a couple of words where a forced division can land.
type forceCut struct {
	left, right int
	score       int
}

// forceDivision divides source at the best word boundary that leaves the first
// part under the cap. It never divides inside a code span, a link, a quotation,
// a parenthesis or bold text.
func forceDivision(source, masked string) (string, bool) {
	// The tags come from the whole sentence, because a fragment parsed alone reads "a faithful" as a noun.
	whole := syntax.Parse(masked, nil)
	for _, strict := range []bool{true, false} {
		best, bestScore := "", 0
		for _, c := range candidates(source, masked, strict) {
			left := closeHead(source[:c.left])
			if cutsAside(masked, c.left, c.right) {
				continue
			}
			right, opened := openRest(source, masked, whole, c)
			if right == "" || !divides(left, right) || !closesWhole(source[:c.left], seamBefore(source, c.left), whole) {
				continue
			}
			// A rest that opens a clause of its own reads best.
			if score := c.score + opened; best == "" || score > bestScore {
				best, bestScore = left+" "+right, score
			}
		}
		if best != "" {
			return best, true
		}
	}
	return source, false
}

// closeHead ends the first part of a division as a sentence. A part that
// already ends on a stop keeps it.
func closeHead(head string) string {
	head = strings.TrimRight(head, " ,;:—–-")
	if strings.HasSuffix(head, ".") || strings.HasSuffix(head, "!") || strings.HasSuffix(head, "?") {
		return head
	}
	return head + "."
}

// divides reports whether Check reads left as a sentence of its own under the
// cap. It reads only the start of right, because the rest of it is unchanged.
func divides(left, right string) bool {
	sentences := Sentences(checkMask(left + " " + opening(right)))
	if len(sentences) < 2 {
		return false
	}
	words := WordCount(sentences[0])
	return words <= SentenceWordCap && words == WordCount(checkMask(left))
}

// inBold reports whether byte p of a sentence sits inside bold text. A bold
// run can open in an earlier sentence, so a first marker that closes a word
// means the sentence opens in bold.
func inBold(source string, p int) bool {
	at := strings.Index(source, "**")
	bold := at > 0 && !unicode.IsSpace(rune(source[at-1])) && (at+2 == len(source) || !isWordByte(source[at+2]))
	for at >= 0 && at < p {
		bold = !bold
		next := strings.Index(source[at+2:], "**")
		if next < 0 {
			break
		}
		at += 2 + next
	}
	return bold
}

// phraseReach is how many words past the cap the phrase parse reads.
const phraseReach = 8

// phraseSpans answers the inside of each noun phrase and verb group the parser
// finds where a cut can land, and where each finite verb starts. A cut inside
// a phrase leaves "a detached." behind.
func phraseSpans(masked string, ends []int) ([][]int, set.Set[int]) {
	finite := set.New[int]()
	if len(ends) == 0 {
		return nil, finite
	}
	head := masked[:ends[min(SentenceWordCap+phraseReach, len(ends)-1)]]
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
	}
	return out, finite
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
func candidates(source, masked string, strict bool) []forceCut {
	off := verbatimSpan.FindAllStringIndex(source, -1)
	off = append(off, quotedSpans(source)...)
	off = append(off, parenthetical.FindAllStringIndex(masked, -1)...)
	off = append(off, outerParens(masked)...)
	off = append(off, linkText.FindAllStringIndex(masked, -1)...)
	ends := wordEnds(masked)
	verbs := set.New[int]()
	if strict {
		spans, finite := phraseSpans(masked, ends)
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
		if leftWords > SentenceWordCap {
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
		out = append(out, forceCut{left: p, right: q, score: leftWords - penalty})
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

// How well each way of opening the rest reads, added to a cut's score.
const (
	opensOwnClause = 6
	opensWithVerb  = 3
)

// openRest writes the words after a cut as a sentence of their own, and says
// how well it reads. A clause that names its own subject opens as it is, and
// so does an imperative after an imperative. A finite verb after a conjunction
// gets the subject again. Any other rest is no sentence, and openRest answers "".
func openRest(source, masked string, whole *syntax.Sentence, c forceCut) (string, int) {
	rest := strings.TrimLeft(source[c.right:], " —–-,;:")
	restMasked := masked[len(masked)-len(rest):]
	if rest == "" {
		return "", 0
	}
	seam := seamBefore(source, c.left)
	opener, conjunction := "", ""
	if word := strings.ToLower(firstToken.FindString(rest)); word != "" {
		if connector, ok := connectors[word]; ok {
			cut := len(firstToken.FindString(rest))
			trimmed := strings.TrimLeft(rest[cut:], " ")
			if trimmed == "" {
				return "", 0
			}
			rest = strings.TrimLeft(trimmed, " —–-,;:")
			if rest == "" {
				return "", 0
			}
			restMasked = restMasked[len(restMasked)-len(rest):]
			opener, conjunction = connector, word
		}
	}
	// A cut between words with no mark and no conjunction lands inside a clause.
	if seam == "" && conjunction == "" {
		return "", 0
	}
	// A capital renames an identifier written in lower case, and Check reads no sentence start there.
	if lowerIdentifier(firstToken.FindString(rest)) {
		return "", 0
	}
	s := syntax.Parse(opening(restMasked), nil)
	first := wordFrom(whole, len(source)-len(rest))
	if len(s.Words) == 0 || first < 0 {
		return "", 0
	}
	switch tag := whole.Words[first].Tag; {
	case (tag == "VB" || tag == "VBP") && opensImperativeMain(masked[:c.left]) && opensImperative(restMasked):
		// An imperative joins only another imperative, and never after a bare comma, where it is an item of a list.
		if seam == "," && conjunction == "" || conjunction == "so" {
			return "", 0
		}
		return joinOpener(opener, rest), opensOwnClause
	case tag == "VBZ" || tag == "VBP" || tag == "VBD" || tag == "MD":
		// Verb groups after a bare comma are a list, and a list never divides.
		if conjunction == "" || conjunction == "so" || listsVerbs(masked[:c.left]) {
			return "", 0
		}
		subject := subjectFor(source, masked, c, tag)
		if subject == "" {
			return "", 0
		}
		return joinOpener(opener, subject+" "+rest), opensWithVerb
	case opensClause(s) && opensSubject(whole, first) && agrees(s, *s.Clauses[0].Subject, *s.Clauses[0].Verb) &&
		verbOfSubject(s, *s.Clauses[0].Subject, *s.Clauses[0].Verb):
		// A so with no comma before it, or after an instruction, states a purpose.
		if conjunction == "so" && (seam != "," || opensImperativeMain(masked[:c.left]) || instructs(whole)) {
			return "", 0
		}
		// After a list, ", and" adds the last item to it: "rules, hooks, and `ask` rules apply".
		if seam == "," && listsVerbs(masked[:c.left]) {
			return "", 0
		}
		return joinOpener(opener, rest), opensOwnClause
	}
	return "", 0
}

// openingBytes bounds how much of the rest the parser reads. Only its first clause decides how the rest opens.
const openingBytes = 240

// opening answers the start of text, cut back to a blank.
func opening(text string) string {
	if len(text) <= openingBytes {
		return text
	}
	if at := strings.LastIndexByte(text[:openingBytes], ' '); at > 0 {
		return text[:at]
	}
	return text
}

// subjectFor names the subject of the words before the cut again, for a verb
// that opens the rest. A short subject repeats. A long one becomes a pronoun
// that agrees with the verb. With no subject to name, it answers "".
func subjectFor(source, masked string, c forceCut, tag string) string {
	s := syntax.Parse(masked[:c.left], nil)
	// Only the clause right before the cut shares its subject with the verb after it.
	for _, clause := range s.Clauses[max(len(s.Clauses)-1, 0):] {
		if clause.Depth != 0 || clause.Subject == nil || clause.Verb == nil {
			continue
		}
		subject := *clause.Subject
		if subject.Last >= clause.Verb.First || !opensSubject(s, subject.First) {
			return ""
		}
		head := s.Words[subject.Head]
		if head.Tag == "PRP" {
			return lowerOpening(head.Text)
		}
		if subject.Last-subject.First < restateLimit && !subject.Coordinated {
			text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
			if first := s.Words[subject.First]; first.Tag == "DT" || first.Tag == "PRP$" {
				text = lowerOpening(text)
				if det := strings.ToLower(first.Text); det == "a" || det == "an" {
					text = "the" + text[len(det):]
				}
			}
			return text
		}
		if tag == "VBP" || tag != "VBZ" && s.Plural(subject) {
			return "they"
		}
		if s.Person(subject) {
			return ""
		}
		return "it"
	}
	return ""
}

// lowerOpening writes the first letter in lower case.
func lowerOpening(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(first)) + s[width:]
}

// lastField answers the last blank-separated word of s.
func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
