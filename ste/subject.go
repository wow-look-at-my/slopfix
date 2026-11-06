// quantifiers open a subject that picks members out of a set.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// quantifiers open a subject that picks members out of a set.
var quantifiers = set.Of("every", "each", "all", "any", "some", "both")

// pluralSubjectAt reports a word in -s at i that opens a subject the tagger
// read as a verb. "such as" after it names examples of a noun, as in "models
// such as Prisma cache". A plural verb after it, past nothing but
// adjectives, prepositions and nouns, agrees with a plural noun too.
func pluralSubjectAt(s *syntax.Sentence, i int) bool {
	if i < 0 || i >= len(s.Words) || s.Words[i].Tag != "VBZ" && s.Words[i].Tag != "NNS" || !strings.HasSuffix(s.Words[i].Lower(), "s") {
		return false
	}
	if i+2 < len(s.Words) && s.Words[i+1].Lower() == "such" && s.Words[i+2].Lower() == "as" {
		return true
	}
	for j := i + 1; j < len(s.Words); j++ {
		switch s.Words[j].Tag {
		case "VBP", "MD":
			return true
		case "JJ", "IN", "NN", "NNS", "NNP", "NNPS", "CD", "POS":
			continue
		}
		return false
	}
	return false
}

// restates reports a subject short enough to write again after a division, and
// not opened by a quantifier.
func restates(s *syntax.Sentence, subject syntax.Phrase) bool {
	return subject.Last-subject.First < restateLimit && !subject.Coordinated && !quantifiers.Contains(s.Words[subject.First].Lower())
}

// subjectDivision writes a long subject as a sentence of its own: "A reader
// arriving at X still deserves Y." becomes "Consider a reader arriving at X.
// That reader still deserves Y." The noun the subject names carries the rest.
func subjectDivision(source string, whole *syntax.Sentence, limit int) (string, bool) {
	// The subject runs from the opening determiner to the first finite verb. The parser often names a later noun as the subject of a long one.
	if len(whole.Words) == 0 || whole.Words[0].Tag != "DT" {
		return source, false
	}
	verb := -1
	for i, w := range whole.Words {
		// A relative word ahead of the verb makes the verb the relative clause's own: "a sentence that will not fit".
		if w.Text == "," || w.Text == ":" || w.Text == ";" || w.Text == "(" || w.Tag == "WDT" || w.Tag == "WP" || w.Tag == "WP$" || w.Tag == "WRB" || w.Lower() == "that" {
			return source, false
		}
		if finiteVerbTag(w.Tag) || w.Tag == "MD" {
			verb = i
			break
		}
		// A noun followed by a new subject opens a relative clause with no relative
		// word: "a file the rule reports nothing in".
		if i > 0 && strings.HasPrefix(whole.Words[i-1].Tag, "NN") && (w.Tag == "DT" || w.Tag == "PRP" || w.Tag == "PRP$") {
			return source, false
		}
	}
	if verb < restateLimit+1 {
		return source, false
	}
	var head syntax.Word
	found := false
	for _, ph := range whole.Phrases {
		if ph.Kind == syntax.NounPhrase && ph.First == 0 {
			head, found = whole.Words[ph.Head], true
		}
	}
	if !found || !strings.HasPrefix(head.Tag, "NN") || head.Tag == "NNP" {
		return source, false
	}
	first := verb
	// An adverb right before the verb goes with the verb: "still deserves".
	for first > 1 && whole.Words[first-1].Tag == "RB" {
		first--
	}
	end := whole.Words[first].Start
	subject := strings.TrimRight(source[:end], " ")
	if WordCount(checkMask(subject))+1 > limit {
		return source, false
	}
	det := "That"
	if head.Tag == "NNS" || head.Tag == "NNPS" {
		det = "Those"
	}
	return "Consider " + lowerFirst(subject) + ". " + det + " " + nounText(source, head) + " " + source[end:], true
}
