// tails.go closes a comment that stops mid-thought.
//
// The words that open something live in rules/comment-tails.xml as a class.
// The English sentence parser finds the sentence the repair drops. Neither the
// vocabulary nor the boundary is spelled here.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// tailsTable is what rules/ says for="comment-tails".
var tailsTable = table.MustLoad(rules.FS, "comment-tails")

// danglingWords answers the class that decides whether a comment finished.
func danglingWords() []string { return tailsClass("dangling") }

// tailsClass answers a named word class of rules/comment-tails.xml.
func tailsClass(name string) []string {
	for _, c := range tailsTable.Classes {
		if c.Name == name {
			return c.Words
		}
	}
	panic("commentfix: rules/ names no class " + name)
}

// IDTail names the rule, the way a compiler names a warning.
const IDTail = "comments/tail"

// CheckTails reports every comment paragraph that stops mid-thought. Fix closes
// each, so a finding here is a finding --fix answers.
func CheckTails(filename, src string) []LengthHit {
	defer trace.Phase("rule/comments-tail")()
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	var hits []LengthHit
	lines := strings.Split(src, "\n")
	for _, para := range paragraphsOf(lines, runs) {
		if !stopsMidThought(para.prose) {
			continue
		}
		hits = append(hits, LengthHit{
			ID:         IDTail,
			Tell:       "the comment stops on a word that opens what is no longer there",
			Sentence:   para.prose,
			Line:       para.lines[0] + 1,
			Repairable: CloseProse(para.prose) != para.prose,
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
	// A comment that reached its full stop said what it meant to, whatever.
	last := words[len(words)-1]
	if endsSentence(last) || !dangling.Contains(strings.ToLower(trimWord(last))) {
		return prose
	}
	kept := append([]string{}, sentences[:len(sentences)-1]...)
	// The clause that opened what is missing goes with it, so the sentence ends
	// where it last said something whole.
	if clause := lastClause(words); clause != "" && standsAlone(clause) {
		kept = append(kept, clause)
	} else if head, ok := wholeHead(words); ok {
		kept = append(kept, head)
	}
	// With nothing whole left, the fragment says nothing a reader can use, so it goes.
	return strings.TrimSpace(strings.Join(kept, " "))
}

// wholeHead drops words from the end of a fragment until what is left stands
// as a sentence, and closes it. It never ends on a word that opens a phrase.
func wholeHead(words []string) (string, bool) {
	for n := len(words) - 1; n >= minimumHead; n-- {
		last := strings.ToLower(trimWord(words[n-1]))
		if dangling.Contains(last) || auxiliary.Contains(last) {
			continue
		}
		head := strings.TrimRight(strings.Join(words[:n], " "), " ,;:—–-")
		if standsAlone(head + ".") {
			return head + ".", true
		}
	}
	return "", false
}

// minimumHead is the fewest words a head kept from a fragment holds.
const minimumHead = 3

// auxiliary words carry a verb that must follow them, so a head never ends on one.
var auxiliary = set.Of("is", "are", "was", "were", "be", "been", "has", "have", "had", "do", "does", "did",
	"can", "will", "must", "should", "may", "might", "would", "could", "it", "they", "which", "that")

// stopsMidThought reports prose whose last sentence ends on a word that opens
// what is missing.
func stopsMidThought(prose string) bool {
	sentences := ste.Sentences(prose)
	if len(sentences) == 0 {
		return false
	}
	words := strings.Fields(sentences[len(sentences)-1])
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	return !endsSentence(last) && dangling.Contains(strings.ToLower(trimWord(last)))
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

// standsAlone reports a clause that reads as a sentence. A doc comment opens on
// the name it documents. The tagger can read that name as a verb. The name is also
// read as the pronoun that takes its place: "Close answers the gate".
func standsAlone(clause string) bool {
	if ste.StandsAlone(clause) {
		return true
	}
	first, rest, ok := strings.Cut(strings.TrimSpace(clause), " ")
	if !ok || first == "" || !unicode.IsUpper(rune(first[0])) || strings.ContainsAny(first, ",;:()") || openers.Contains(strings.ToLower(first)) {
		return false
	}
	return ste.StandsAlone("It " + rest)
}

// openers are the words that open a clause or a phrase, never a name.
var openers = set.Of("if", "when", "because", "since", "while", "unless", "until", "although", "though",
	"after", "before", "so", "and", "but", "or", "as", "where", "whether", "once", "the", "a", "an",
	"this", "that", "these", "those", "each", "every", "all", "some", "no", "for", "with", "to", "in",
	"on", "at", "by", "from", "of", "then", "only", "also")

// trimWord drops the punctuation a word carries, leaving the word itself.
func trimWord(w string) string {
	return strings.Trim(w, ".,;:!?)\"'")
}
