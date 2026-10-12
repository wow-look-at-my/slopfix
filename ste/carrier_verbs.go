package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// mainVerb answers the verb group of the main clause that holds the words
// before byte at.
func mainVerb(s *syntax.Sentence, at int) (syntax.Phrase, bool) {
	for _, c := range s.Clauses {
		if c.Depth == 0 && c.Verb != nil && s.Words[c.Verb.First].Start < at {
			return *c.Verb, true
		}
	}
	return syntax.Phrase{}, false
}

// participleAfterDash reports a verb at word i that takes "is" after "It". A
// present participle always does. A past form does when the tagger names it a
// participle, or when the main verb is present, so the past form is no tense.
func participleAfterDash(s *syntax.Sentence, i int, main syntax.Phrase, hasMain bool) bool {
	switch s.Words[i].Tag {
	case "VBN", "VBG":
		return true
	case "VBD":
		if !hasMain {
			return false
		}
		tag := s.Words[main.Head].Tag
		return tag == "VBZ" || tag == "VBP"
	}
	return false
}

// verbAt reports a verb at word i, finite or bare.
func verbAt(s *syntax.Sentence, i int) bool {
	return i < len(s.Words) && (strings.HasPrefix(s.Words[i].Tag, "VB") || s.Words[i].Tag == "MD")
}

// negated reports a verb group that holds a negation, or one right after it.
func negated(s *syntax.Sentence, verb syntax.Phrase) bool {
	for i := max(verb.First-1, 0); i <= min(verb.Last+1, len(s.Words)-1); i++ {
		if negations.Contains(s.Words[i].Lower()) {
			return true
		}
	}
	return false
}
