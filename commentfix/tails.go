// tails.go closes a comment that stops mid-thought.
//
// The words that open something live in rules/comment-tails.xml as a class, and
// the sentence the repair drops is found by the English sentence parser, so
// neither the vocabulary nor the boundary is spelled here.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/table"
)

// tailsTable is what rules/ says for="comment-tails".
var tailsTable = table.MustLoad(rules.FS, "comment-tails")

// danglingWords answers the class that decides whether a comment finished.
func danglingWords() []string {
	for _, c := range tailsTable.Classes {
		if c.Name == "dangling" {
			return c.Words
		}
	}
	panic("commentfix: rules/ names no class dangling")
}

// CloseProse closes a comment that stops on an opening word.
func CloseProse(prose string) string {
	sentences := ste.Sentences(prose)
	if len(sentences) == 0 {
		return prose
	}
	last := sentences[len(sentences)-1]
	closed := closeTail(strings.Fields(last))
	if strings.Join(closed, " ") == strings.TrimSpace(last) {
		return prose
	}
	// A fragment after a finished sentence leaves nothing worth keeping, so
	// the sentences before it are the whole answer.
	if len(closed) == 0 {
		return strings.TrimSpace(strings.Join(sentences[:len(sentences)-1], " "))
	}
	kept := append(append([]string{}, sentences[:len(sentences)-1]...), strings.Join(closed, " "))
	return strings.TrimSpace(strings.Join(kept, " "))
}
