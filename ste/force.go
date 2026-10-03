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
// lands between words, near the cap, and opens the rest so it still reads.

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
	for range len(strings.Fields(prose)) + 1 {
		next, divided := forceNext(prose)
		if !divided {
			return prose
		}
		prose = next
	}
	return prose
}

// forceNext divides the earliest over-cap sentence, as Check reads it.
func forceNext(prose string) (string, bool) {
	masked := checkMask(prose)
	for _, span := range sentenceSpans(masked) {
		start, end := span[0], span[1]
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

// checkMask writes a filler word over each span Check strips, byte for byte.
// The sentences and the word counts then match Check, and an offset into the
// mask is an offset into the prose.
func checkMask(prose string) string {
	out := []byte(prose)
	spans := verbatimSpan.FindAllStringIndex(prose, -1)
	spans = append(spans, quotation.FindAllStringIndex(prose, -1)...)
	for _, span := range spans {
		fillWord(out[span[0]:span[1]])
	}
	return string(out)
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
	total := WordCount(masked)
	for _, strict := range []bool{true, false} {
		for _, c := range candidates(source, masked, strict) {
			left := strings.TrimRight(source[:c.left], " ,;:—–-") + "."
			right := openRest(source, masked, c)
			if right == "" || WordCount(checkMask(right)) >= total {
				continue
			}
			return left + " " + right, true
		}
	}
	return source, false
}

// candidates answers every admissible cut, best first. A strict pass also keeps
// each part off a word that leaves it hanging.
func candidates(source, masked string, strict bool) []forceCut {
	off := verbatimSpan.FindAllStringIndex(source, -1)
	off = append(off, quotation.FindAllStringIndex(source, -1)...)
	off = append(off, parenthetical.FindAllStringIndex(masked, -1)...)
	off = append(off, linkText.FindAllStringIndex(masked, -1)...)
	var out []forceCut
	for _, gap := range gapRun.FindAllStringIndex(source, -1) {
		p, q := gap[0], gap[1]
		if p == 0 || q == len(source) || insideAny(off, p) {
			continue
		}
		if strings.Count(source[:p], "**")%2 != 0 {
			continue
		}
		leftWords, rightWords := WordCount(masked[:p]), WordCount(masked[q:])
		if leftWords < 1 || leftWords > SentenceWordCap || rightWords < 1 {
			continue
		}
		head := strings.TrimRight(source[:p], " ")
		last := strings.ToLower(strings.Trim(lastField(head), ".,;:!?*_\"'`()[]“”‘’"))
		next := strings.ToLower(strings.Trim(firstToken.FindString(source[q:]), ".,;:!?*_\"'`()[]“”‘’"))
		if strict && (leftWords < minimumHalf || rightWords < minimumHalf || forceDangling.Contains(last) || forceBound.Contains(next)) {
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

// openRest writes the words after a cut as a sentence of their own. A clause
// that names its own subject opens as it is. A verb gets the subject again,
// and anything else opens with "This is".
func openRest(source, masked string, c forceCut) string {
	rest, restMasked := source[c.right:], masked[c.right:]
	opener := ""
	if word := strings.ToLower(firstToken.FindString(rest)); word != "" {
		if connector, ok := connectors[word]; ok {
			cut := len(firstToken.FindString(rest))
			trimmed := strings.TrimLeft(rest[cut:], " ")
			if trimmed == "" {
				return ""
			}
			rest, restMasked = trimmed, restMasked[len(restMasked)-len(trimmed):]
			opener = connector
		}
	}
	if strings.EqualFold(firstToken.FindString(rest), "which") {
		return joinOpener(opener, "this"+rest[len("which"):])
	}
	s := syntax.Parse(restMasked, nil)
	if len(s.Words) == 0 {
		return ""
	}
	if opensClause(s) {
		return joinOpener(opener, rest)
	}
	if tag := s.Words[0].Tag; tag == "VBZ" || tag == "VBP" || tag == "VBD" || tag == "MD" {
		return joinOpener(opener, subjectFor(source, masked, c, tag)+" "+rest)
	}
	return joinOpener(opener, "this is "+rest)
}

// opensClause reports a parse whose first clause starts with its own subject
// and carries a finite verb.
func opensClause(s *syntax.Sentence) bool {
	if len(s.Clauses) == 0 {
		return false
	}
	c := s.Clauses[0]
	return c.Subject != nil && c.Verb != nil && c.Subject.First == 0
}

// subjectFor names the subject of the words before the cut again, for a verb
// that opens the rest. A short subject repeats. A long one becomes a pronoun
// that agrees with the verb.
func subjectFor(source, masked string, c forceCut, tag string) string {
	s := syntax.Parse(masked[:c.left], nil)
	for _, clause := range s.Clauses {
		if clause.Depth != 0 || clause.Subject == nil {
			continue
		}
		subject := *clause.Subject
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
		return "it"
	}
	if tag == "VBP" {
		return "they"
	}
	return "this"
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
