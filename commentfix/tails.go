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
	"github.com/wow-look-at-my/slopfix/treecomments"
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

// IDTail names the rule, the way a compiler names a warning.
const IDTail = "comments/tail"

// CheckTails reports every comment paragraph that stops mid-thought. Fix closes
// each, so a finding here is a finding --fix answers.
func CheckTails(filename, src string) []LengthHit {
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	var hits []LengthHit
	lines := strings.Split(src, "\n")
	for _, para := range paragraphsOf(lines, runs) {
		if CloseProse(para.prose) == para.prose {
			continue
		}
		hits = append(hits, LengthHit{
			ID:         IDTail,
			Tell:       "the comment stops on a word that opens what is no longer there",
			Sentence:   para.prose,
			Line:       para.lines[0] + 1,
			Repairable: true,
		})
	}
	return hits
}

// CloseProse closes a comment that stops on an opening word.
func CloseProse(prose string) string {
	sentences := ste.Sentences(prose)
	if len(sentences) == 0 {
		return prose
	}
	words := strings.Fields(sentences[len(sentences)-1])
	if len(words) == 0 {
		return prose
	}
	// A comment that reached its full stop said what it meant to, whatever the
	// word standing before it: "the file the script was read from." is whole.
	last := words[len(words)-1]
	if endsSentence(last) || !dangling.Contains(strings.ToLower(trimWord(last))) {
		return prose
	}
	kept := append([]string{}, sentences[:len(sentences)-1]...)
	// The clause that opened what is missing goes with it, so the sentence ends
	// where it last said something whole.
	if clause := lastClause(words); clause != "" {
		kept = append(kept, clause)
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

// lastClause answers the sentence up to the punctuation that opened the clause
// the cut took away, closed with a period. It answers "" when the whole
// sentence was that clause.
func lastClause(words []string) string {
	for i := len(words) - 1; i >= 0; i-- {
		if !strings.HasSuffix(words[i], ",") && !strings.HasSuffix(words[i], ";") &&
			!strings.HasSuffix(words[i], ":") {
			continue
		}
		joined := strings.Join(words[:i+1], " ")
		return joined[:len(joined)-1] + "."
	}
	return ""
}

// trimWord drops the punctuation a word carries, leaving the word itself.
func trimWord(w string) string {
	return strings.Trim(w, ".,;:!?)\"'")
}
