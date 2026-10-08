package ste

import (
	"github.com/wow-look-at-my/slopfix/syntax"
)

// nounText answers the source text of a word, a whole code span where the
// word is the filler the mask wrote over one.
func nounText(source string, w syntax.Word) string {
	return source[outsideSpans(source, w.Start, false):outsideSpans(source, w.End, true)]
}

// restate writes "That <noun> is <rest>", or "Those <nouns> are <rest>".
func restate(source string, noun syntax.Word, rest string) string {
	det, be := "That", "is"
	if noun.Tag == "NNS" || noun.Tag == "NNPS" {
		det, be = "Those", "are"
	}
	return det + " " + nounText(source, noun) + " " + be + " " + rest
}

// restateBare writes "That <noun> <rest>", where rest opens with the verb of a
// relative clause that already agrees with the noun.
func restateBare(source string, noun syntax.Word, rest string) string {
	det := "That"
	if noun.Tag == "NNS" || noun.Tag == "NNPS" {
		det = "Those"
	}
	return det + " " + nounText(source, noun) + " " + rest
}
