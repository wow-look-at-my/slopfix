// A division ends the left sentence at leftEnd.

package ste

import (
	"regexp"
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// A division ends the left sentence at leftEnd. The right sentence opens with
// opener, then the source from rightStart.
type division struct {
	leftEnd, rightStart int
	opener              string
	// closer is the object a division writes back at the end of the rest: ", which a list does not show" becomes "A list does not show that."
	closer string
	// comma is the byte offset where the rest takes a comma after an opening participle phrase: "Dropped, it leaves". Zero means none.
	comma int
}

func participleComma(s *syntax.Sentence, c syntax.Clause) int {
	p := c.Link + 1
	if p+2 >= len(s.Words) || s.Words[p].Tag != "VBN" && s.Words[p].Tag != "VBD" {
		return -1
	}
	if c.Subject == nil {
		// "and dropped it leaves": the pronoun after the participle is the subject.
		if s.Words[p+1].Tag == "PRP" && finiteAt(s, p+2) {
			return p + 1
		}
		return -1
	}
	if c.Subject.First > p+1 {
		return c.Subject.First
	}
	return -1
}

// restOf answers the rest of the source a division writes, with its comma.
func restOf(source string, d division) string {
	if d.comma <= d.rightStart {
		return source[d.rightStart:]
	}
	return source[d.rightStart:d.comma] + "," + source[d.comma:]
}

// dashMark reports a dash, which introduces a clause the sentence before it
// stands without.
func dashMark(text string) bool {
	return text == "--" || text == "—" || text == "–" || text == "-"
}

// connectors map a conjunction to the words that open the sentence after it.
// A period says what and and so say, so both drop.
var connectors = map[string]string{
	"and": "",
	"but": "However,",
	"yet": "However,",
	"so":  "",
	"or":  "Otherwise,",
}

// keepsCodeSpans reports that a division kept each code span whole, in order.
func keepsCodeSpans(source, rewritten string) bool {
	return slices.Equal(codeSpan.FindAllString(source, -1), codeSpan.FindAllString(rewritten, -1))
}

// sentenceSpans answers the byte range of each sentence. It reads the source
// rather than a mask, because a masked code span opens a sentence.
func sentenceSpans(prose string) [][2]int {
	var out [][2]int
	at := 0
	for _, sentence := range Sentences(prose) {
		start := strings.Index(prose[at:], sentence)
		if start < 0 {
			break
		}
		start += at
		at = start + len(sentence)
		out = append(out, [2]int{start, at})
	}
	return out
}

// opaque are the spans the parser reads as names: the data Check hides, a
// parenthetical, and a quotation, whose words belong to somebody else.
func opaque(prose, masked string) [][]int {
	off := offLimits(prose, masked)
	for _, span := range cardinal.QuotedSpans(prose) {
		off = append(off, []int{span.Start, span.End})
	}
	for _, loc := range curlyQuote.FindAllStringIndex(prose, -1) {
		off = append(off, loc)
	}
	return off
}

var curlyQuote = regexp.MustCompile(`“[^”]*”`)

// bestDivision picks the division that leaves the longer half shortest.
func bestDivision(s *syntax.Sentence, source string) (string, bool) {
	best, bestScore := "", -1
	for _, d := range divisions(s, source) {
		// A division inside bold text leaves each half with an unclosed marker.
		if strings.Count(source[:d.leftEnd], "**")%2 != 0 {
			continue
		}
		// A bold run that opens on the conjunction the division drops opens the rest instead: "X **and Y is flagged**" becomes "X. **Y is flagged**".
		bold := ""
		if dropped := strings.TrimSpace(source[d.leftEnd:d.rightStart]); strings.Contains(dropped, "**") {
			if strings.Count(dropped, "**") != 1 || !strings.HasPrefix(dropped, "**") {
				continue
			}
			bold = "**"
		}
		if !admissible(s, source, d) || absolutePhrase(source, d) {
			continue
		}
		left := strings.TrimRight(source[:d.leftEnd], " ,;:—–-") + "."
		opened := openDivision(source, d)
		if opened == "" {
			continue
		}
		right := bold + withCloser(opened, d.closer)
		if WordCount(left) < minimumHalf || WordCount(right) < minimumHalf {
			continue
		}
		// The parse of the whole can hand a clause a verb that belongs elsewhere, so each half is read again on its own.
		if !standsAsSentence(left) {
			continue
		}
		if d.opener == "" && !standsAsSentence(right) {
			continue
		}
		// A noun phrase whose only verbs sit in relative clauses is a fragment: "A number that is true today".
		if !finiteOutsideRelative(syntax.Parse(checkMask(right), nil), 0) {
			continue
		}
		score := max(WordCount(left), WordCount(right))
		if bestScore < 0 || score < bestScore {
			best, bestScore = left+" "+right, score
		}
	}
	return best, bestScore >= 0
}

// coordinateDivision divides at a comma before a conjunction the clause pass
// left inside one clause, as in "X, so Y" or "X, but Y". It is the fallback
// behind the clause boundaries, and each half must read as a sentence.
func coordinateDivision(s *syntax.Sentence, source string) (string, bool) {
	best, bestScore := "", -1
	for _, d := range beforeCoordinate(s, source) {
		if cutsAside(mask(source), d.leftEnd, d.rightStart) {
			continue
		}
		left := strings.TrimRight(source[:d.leftEnd], " ,;:—–-") + "."
		right := openDivision(source, d)
		if WordCount(left) < minimumHalf || WordCount(right) < minimumHalf {
			continue
		}
		if w, ok := lastWordBefore(s, len(source[:d.leftEnd])); ok && danglingTags.Contains(w.Tag) {
			continue
		}
		// A comma before a conjunction is a real boundary, so the head stands even
		// where the whole sentence named no main clause for it.
		if n := wordFrom(s, d.rightStart); n >= 0 {
			if tag := s.Words[n].Tag; tag == "IN" || tag == "TO" {
				continue
			}
		}
		if !standsAsSentence(right) {
			continue
		}
		score := max(WordCount(left), WordCount(right))
		if bestScore < 0 || score < bestScore {
			best, bestScore = left+" "+right, score
		}
	}
	return best, bestScore >= 0
}

// Leading closes a sentence at a clause boundary the parser finds. It keeps the
// longest head that stands as a sentence under the cap, and reports false when
// no boundary gives one.
func Leading(sentence string) (string, bool) {
	masked := mask(sentence)
	s := syntax.Parse(masked, opaque(sentence, masked))
	best := ""
	for _, d := range divisions(s, sentence) {
		if !admissible(s, sentence, d) {
			continue
		}
		head := strings.TrimRight(sentence[:d.leftEnd], " ,;:—–-") + "."
		if n := WordCount(head); n >= minimumHalf && n <= SentenceWordCap && len(head) > len(best) {
			best = head
		}
	}
	return best, best != ""
}

// Cuts answers each byte offset where sentence can close at a clause boundary
// and leave a sentence: the boundaries the division repair admits.
func Cuts(sentence string) []int {
	masked := mask(sentence)
	s := syntax.Parse(masked, opaque(sentence, masked))
	var out []int
	for _, d := range divisions(s, sentence) {
		if admissible(s, sentence, d) {
			out = append(out, d.leftEnd)
		}
	}
	return out
}

// minimumHalf keeps a division from writing a sentence too short to stand alone.
const minimumHalf = 3

// withCloser writes closer in front of the stop that ends rest.
func withCloser(rest, closer string) string {
	if closer == "" {
		return rest
	}
	body := strings.TrimRight(rest, ".!? ")
	return body + " " + closer + rest[len(body):]
}

// joinOpener puts the opener in front of the rest, and capitalizes what opens the sentence.
func joinOpener(opener, rest string) string {
	if opener != "" {
		rest = opener + " " + rest
	}
	return capitalizeOpening(rest)
}
