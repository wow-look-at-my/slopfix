package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// objectRelative reports ", which X does not show" at the end of a sentence: a
// relative clause with a subject of its own whose verb ends the sentence. The
// relative word is the verb's object.
func objectRelative(s *syntax.Sentence, c syntax.Clause) bool {
	if c.Kind != syntax.Relative || !c.Comma || c.Depth != 1 || s.Words[c.Link].Lower() != "which" || c.Subject == nil || c.Verb == nil {
		return false
	}
	last := len(s.Words) - 1
	for last > 0 && punctuationTag(s.Words[last].Tag) {
		last--
	}
	return c.Verb.Last == last && c.Subject.First == c.Link+1 && !strings.HasPrefix(s.Words[last].Tag, "VBN")
}

func punctuationTag(tag string) bool { return tag == "." || tag == "," || tag == ":" }
