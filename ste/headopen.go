package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// headOpen reports words before word end that end inside an unfinished clause.
// One is a subordinator with no finite verb after it: "reports whether the
// words". The other is a noun with a new subject and no verb after them:
// "answers the row the earliest node".
func headOpen(s *syntax.Sentence, end int) bool {
	// Each clause still waiting for its verb, innermost last. A finite verb closes the innermost.
	var open []int
	for i := 0; i < end && i < len(s.Words); i++ {
		w := s.Words[i]
		switch {
		case finiteVerbTag(w.Tag) || w.Tag == "MD":
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		case len(open) > 0 && i > open[len(open)-1]+1 && w.Tag == "NNS" && s.Words[i-1].Tag == "NN" && i+1 < len(s.Words) && s.Words[i+1].Tag == "IN":
			// The tagger reads the verb of the new subject as a plural noun: "a word the actor of a clause stops in front of".
			open = open[:len(open)-1]
		case clauseOpeners.Contains(w.Lower()) && i > 0:
			open = append(open, i)
		case i > 0 && s.Words[i-1].Tag == "IN" && (w.Tag == "WDT" || w.Tag == "WP"):
			// "in which the lines": a relative clause after a preposition names its own subject, and waits for its verb.
			open = append(open, i)
		case i > 0 && strings.HasPrefix(s.Words[i-1].Tag, "NN") && (w.Tag == "WDT" || w.Lower() == "that") && i+1 < len(s.Words) && finiteVerbTag(s.Words[i+1].Tag):
			// "lines that are": the relative verb follows at once, and closes this clause and no other.
			open = append(open, i)
		case i > 0 && strings.HasPrefix(s.Words[i-1].Tag, "NN") && (w.Tag == "DT" || w.Tag == "PRP"):
			open = append(open, i)
		}
	}
	return len(open) > 0
}
