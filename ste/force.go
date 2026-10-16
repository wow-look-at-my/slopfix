package ste

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

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
func forceSentenceCap(prose string, d capSpec) string {
	over := overCap(prose, d.cap)
	for range len(strings.Fields(prose)) + 1 {
		next, divided := forceNext(prose, d)
		// A division that leaves as many words past the cap moves nothing, so the loop stops on it.
		if !divided || overCap(next, d.cap) >= over {
			return prose
		}
		prose, over = next, overCap(next, d.cap)
	}
	return prose
}

// overCap counts the words past limit in every sentence Check reads in prose.
func overCap(prose string, limit int) int {
	n := 0
	for _, sentence := range Sentences(checkMask(prose)) {
		n += max(0, WordCount(sentence)-limit)
	}
	return n
}

// forceNext divides the earliest over-cap sentence, as Check reads it.
func forceNext(prose string, d capSpec) (string, bool) {
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
		if WordCount(masked[start:end]) <= d.cap {
			continue
		}
		if rewritten, ok := forceDivision(prose[start:end], masked[start:end], d); ok {
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
	// words is the count Check reads for the part before left.
	words int
}

// forceDivision divides source at the best word boundary that leaves the first
// part under the cap. It never divides inside a code span, a link, a quotation,
// a parenthesis or bold text.
func forceDivision(source, masked string, d capSpec) (string, bool) {
	// The tags come from the whole sentence, because a fragment parsed alone reads "a faithful" as a noun.
	whole := syntax.Parse(masked, nil)
	for _, strict := range []bool{true, false} {
		best, bestScore := "", 0
		for _, c := range candidates(source, masked, strict, d.cap) {
			if cutsAside(masked, c.left, c.right) || splitsObject(whole, c) {
				continue
			}
			head := source[:c.left]
			right, opened := openRest(source, masked, whole, c)
			left := closeHead(head)
			seam := seamBefore(source, c.left)
			// ", so" joins whole clauses, so the comma before it ends one wherever it sits.
			if seam == "," && strings.HasPrefix(strings.ToLower(strings.TrimLeft(source[c.right:], " ")), "so ") {
				seam = "so"
			}
			// A fragment head behind "This is" closes as a sentence of its own.
			accept := func(h, r string) bool {
				joined := closeHead(h) + " " + r
				return r != "" && divides(closeHead(h), r, d.cap) &&
					(h == fragmentHead(source[:c.left]) || closesWhole(source[:c.left], seam, whole) || closesPhrase(source[:c.left], whole)) &&
					overCap(joined, d.cap) < overCap(source, d.cap)
			}
			if !accept(head, right) {
				// The grammatical rest was refused, so the carrier division, which restates the main clause, gets its turn.
				ch, cr, co := carrierOrNone(source, whole, c)
				cl := closeHead(ch)
				if cr == "" || !divides(cl, cr, d.cap) || overCap(cl+" "+cr, d.cap) >= overCap(source, d.cap) {
					continue
				}
				head, right, opened, left = ch, cr, co, cl
			}
			// A rest that opens a clause of its own reads best.
			if score := c.score + opened; best == "" || score > bestScore {
				best, bestScore = left+" "+right, score
			}
		}
		// A division that leaves as many words over the cap moves nothing, and
		// forceSentenceCap would revert it.
		if best != "" && overCap(best, d.cap) < overCap(source, d.cap) {
			return best, true
		}
	}
	if d.reorder {
		if out, ok := reorderDependent(source, whole); ok && overCap(out, d.cap) < overCap(source, d.cap) {
			return out, true
		}
		if out, ok := subjectDivision(source, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
			return out, true
		}
	}
	if out, ok := fragmentDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := markDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := coordinateDivision(whole, source); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := becauseDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	// A pair of dashes is one aside. It divides whole, with the aside kept
	// behind a comma, before the punctuation cut reads the closing dash.
	if out, ok := asideDivision(source, masked, d.cap); ok {
		return out, true
	}
	if out, ok := punctuationDivision(source, masked, whole, d.cap); ok && overCap(out, d.cap) < overCap(source, d.cap) {
		return out, true
	}
	if out, ok := hardDivision(source, masked, d.cap); ok || !d.every {
		return out, ok
	}
	if out, ok := standingMarkDivision(source, masked, d.cap); ok {
		return out, true
	}
	return anyDivision(source, masked, d.cap)
}

// hardDivision divides a sentence that no grammatical division reads. It keeps
// the longest run of leading words under the cap. It closes that run as a
// sentence and opens the rest with a capital. The cap rule then leaves no
// finding standing.
//
// It never ends a half on a word that needs the next one. It never cuts inside
// a code span, a link, a quotation or a parenthesis. candidates answers only
// the gaps outside those spans, so the cut lands between whole words.
func hardDivision(source, masked string, limit int) (string, bool) {
	ends := wordEnds(masked)
	if len(ends) <= limit || len(ends) < 2*minimumHalf {
		return source, false
	}
	whole := syntax.Parse(masked, nil)
	best, bestWords, bestRank := forceCut{}, -1, -1
	for _, c := range candidates(source, masked, false, limit) {
		if cutsAside(masked, c.left, c.right) || c.words > limit || c.words < minimumHalf {
			continue
		}
		if len(ends)-c.words < minimumHalf {
			continue
		}
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && danglingTags.Contains(w.Tag) && !predicateAdjective(whole, wordFrom(whole, w.Start)) {
			// A cut inside a noun phrase strands its noun: "no other | thread".
			continue
		}
		// A verb does not close a sentence before the adjective it links to:
		// "the pointer stays. Valid for reads of n bytes" is no sentence pair.
		if w, ok := lastWordBefore(whole, len(source[:c.left])); ok && strings.HasPrefix(w.Tag, "VB") {
			if n := wordsBefore(whole, c.right); n < len(whole.Words) && strings.HasPrefix(whole.Words[n].Tag, "JJ") {
				continue
			}
		}
		// A tail that opens on a finite verb leaves its subject behind:
		// "the pointer. Stays valid for reads of n bytes".
		if n := wordsBefore(whole, c.right); n < len(whole.Words) && finiteVerbTag(whole.Words[n].Tag) {
			continue
		}
		// A relative word binds its clause to the noun in front of it: "the verb. That governs it".
		if n := wordsBefore(whole, c.right); n > 0 && n+1 < len(whole.Words) && relativeAfterNoun(whole, n) {
			continue
		}
		head := strings.TrimRight(source[:c.left], " ,;:—–-")
		// The head must hold a clause of its own, or the cut strands a subject
		// without its verb: "the pointer. Stays valid for reads of n bytes".
		if !holdsFinite(checkMask(head)) && !opensImperative(checkMask(head)) {
			continue
		}
		last := strings.ToLower(strings.Trim(lastField(head), ".,;:!?*_\"'`()[]“”‘’"))
		next := strings.ToLower(strings.Trim(firstToken.FindString(source[c.right:]), ".,;:!?*_\"'`()[]“”‘’"))
		if endsOnPossessive(source, c.left) {
			continue
		}
		if forceDangling.Contains(last) || forceBound.Contains(next) {
			continue
		}
		// Both parts must read as sentences, or the cut strands a fragment such
		// as "writes. Through it" -- and the start of the rest decides that.
		if !standsAsSentence(closeHead(source[:c.left])) ||
			!standsAsSentence(opening(hardRest(source, c.right))) {
			continue
		}
		// A rest that opens on a preposition and holds a finite verb only inside a relative clause has no main clause: "With the verb that governs it, laid out".
		if rs := syntax.Parse(opening(hardRest(source, c.right)), nil); len(rs.Words) > 0 && (rs.Words[0].Tag == "IN" || rs.Words[0].Tag == "TO") && !finiteOutsideRelative(rs, 0) {
			continue
		}
		// The best seam wins, then the longest leading run. The first cut of a tie is deterministic.
		rank := hardSeamRank(source, whole, c)
		if rank > bestRank || rank == bestRank && c.words > bestWords {
			best, bestWords, bestRank = c, c.words, rank
		}
	}
	if bestWords < 0 {
		best, bestWords = lastResortCut(source, masked, len(ends), limit)
	}
	if bestWords < 0 {
		return source, false
	}
	left := closeHead(source[:best.left])
	right := hardRest(source, best.right)
	if left == "" || right == "" {
		return source, false
	}
	return left + " " + right, true
}

// relativeAfterNoun reports a relative word at word n, such as "that" or
// "which", right behind a noun and in front of a verb. Its clause belongs to
// that noun, so no sentence opens on it.
func relativeAfterNoun(s *syntax.Sentence, n int) bool {
	switch s.Words[n].Lower() {
	case "that", "which", "who":
	default:
		return false
	}
	after := s.Words[n+1].Tag
	return strings.HasPrefix(s.Words[n-1].Tag, "NN") && (strings.HasPrefix(after, "VB") || after == "NNS")
}

// endsOnPossessive reports a cut whose head closes on a possessive. The
// possessive governs the word after it, so a part never ends on one.
func endsOnPossessive(source string, left int) bool {
	last := strings.ToLower(strings.Trim(lastField(strings.TrimRight(source[:left], " ,;:—–-")), ".,;:!?*_\"'`()[]“”‘’"))
	return strings.HasSuffix(last, "'s") || strings.HasSuffix(last, "’s")
}

// lastResortCut answers the cut that keeps the most leading words under the
// cap, for a sentence where no cut leaves both parts reading as sentences. A
// finding that fix leaves standing breaks the guarantee, and an awkward pair
// of sentences does not. The cut still lands between whole words, outside a
// code span, a link, a quotation and a parenthesis.
func lastResortCut(source, masked string, words, limit int) (forceCut, int) {
	best, bestWords := forceCut{}, -1
	whole := syntax.Parse(masked, nil)
	for _, c := range candidates(source, masked, false, limit) {
		if cutsAside(masked, c.left, c.right) || c.words > limit || c.words < 1 || words-c.words < 1 {
			continue
		}
		if endsOnPossessive(source, c.left) {
			continue
		}
		if n := wordsBefore(whole, c.right); n > 0 && n+1 < len(whole.Words) && relativeAfterNoun(whole, n) {
			continue
		}
		if c.words > bestWords {
			best, bestWords = c, c.words
		}
	}
	return best, bestWords
}

// hardRest opens the words after a cut at right as a sentence, past any dash that opened an aside.
func hardRest(source string, right int) string {
	return capitalizeOpening(strings.TrimLeft(source[right:], " —–"))
}

// hardSeamRank grades a cut for hardDivision. A cut that parts a subject from
// its verb, or a verb from what follows it, ranks lowest: "it. Is carrying".
// A cut at a mark, or before a conjunction or a preposition, ranks highest.
func hardSeamRank(source string, whole *syntax.Sentence, c forceCut) int {
	first := wordFrom(whole, c.right)
	if first < 1 || first >= len(whole.Words) {
		return 0
	}
	next, last := whole.Words[first].Tag, whole.Words[first-1].Tag
	if strings.HasPrefix(next, "VB") || next == "MD" || next == "RP" || next == "POS" || strings.HasPrefix(last, "VB") || last == "MD" {
		return 0
	}
	if strings.ContainsAny(source[c.left:c.right], ",;:—–") || strings.HasSuffix(strings.TrimRight(source[:c.left], " "), ",") ||
		next == "CC" || next == "IN" || next == "WDT" || next == "WRB" {
		return 2
	}
	return 1
}

// carrierOrNone is carrierDivision, except for a sentence that opens with an
// infinitive of purpose ("To pull X, use Y"). That sentence reorders behind "Do
// this", and a carrier would restate the purpose clause's verb in place of the
// main one, so it stands down.
func carrierOrNone(source string, whole *syntax.Sentence, c forceCut) (string, string, int) {
	if len(whole.Words) > 1 && whole.Words[0].Lower() == "to" && strings.HasPrefix(whole.Words[1].Tag, "VB") {
		return source[:c.left], "", 0
	}
	return carrierDivision(source, whole, c)
}

// alsoVerb writes the main verb group with "also" before its last verb, so
// "must ensure" restates as "must also ensure".
func alsoVerb(source string, s *syntax.Sentence, verb syntax.Phrase) string {
	text := source[s.Words[verb.First].Start:s.Words[verb.Last].End]
	at := s.Words[verb.Last].Start - s.Words[verb.First].Start
	return text[:at] + "also " + text[at:]
}

// splitsObject reports a cut between a finite verb and the noun phrase right
// after it, which is the verb's object: "a sentence names | an item".
func splitsObject(s *syntax.Sentence, c forceCut) bool {
	next := wordFrom(s, c.right)
	// A mark between the verb and the words after it ends the verb's phrase.
	if next < 1 {
		return false
	}
	// The tagger reads "names" or "reads" as a plural noun. It is the verb when the words before it hold no other.
	prev := s.Words[next-1].Tag
	if !finiteVerbTag(prev) && !(prev == "NNS" && !finiteBetween(s, 0, next-1)) {
		return false
	}
	switch tag := s.Words[next].Tag; {
	case tag == "DT" || tag == "PRP$" || tag == "CD" || strings.HasPrefix(tag, "JJ") || strings.HasPrefix(tag, "NN"):
		return true
	}
	return false
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

// divides reports whether Check reads left as a sentence of its own under
// limit. It reads only the start of right, because the rest of it is unchanged.
func divides(left, right string, limit int) bool {
	sentences := Sentences(checkMask(left + " " + opening(right)))
	if len(sentences) < 2 {
		return false
	}
	words := WordCount(sentences[0])
	return words <= limit && words == WordCount(checkMask(left))
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
func phraseSpans(masked string, ends []int, limit int) ([][]int, set.Set[int]) {
	finite := set.New[int]()
	if len(ends) == 0 {
		return nil, finite
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
func candidates(source, masked string, strict bool, limit int) []forceCut {
	off := verbatimSpan.FindAllStringIndex(source, -1)
	off = append(off, quotedSpans(source)...)
	off = append(off, parenthetical.FindAllStringIndex(masked, -1)...)
	off = append(off, outerParens(masked)...)
	off = append(off, linkText.FindAllStringIndex(masked, -1)...)
	ends := wordEnds(masked)
	verbs := set.New[int]()
	if strict {
		spans, finite := phraseSpans(masked, ends, limit)
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
		out = append(out, forceCut{left: p, right: q, score: leftWords - penalty, words: leftWords})
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
