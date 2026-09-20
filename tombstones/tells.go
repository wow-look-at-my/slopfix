// tells.go holds what a tombstone looks like on the page.
//
// A property separates it from a comment worth keeping. Its referent is gone:
// the flag, the test, the old spelling it names does not exist any more. Or its
// audience is the reviewer: it argues for the change instead of telling the
// next editor what breaks.
//
// The table matches the surface of each. An entry names the tell so a report
// can say which property failed.
package tombstones

import (
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/english"
)

// deadReferent is the tell referents.go reports.
const deadReferent = "a name nothing in the repository defines"

// IDVolume names the volume cap, whose tell carries a line count instead.
const IDVolume = "tombstones/comment-volume"

// AllIDs names every rule this package reports, for a caller that validates a
// name before running.
func AllIDs() set.Set[string] {
	ids := set.New[string]()
	ids.AddRange(IDVolume, ruleID(deadReferent))
	for _, id := range english.PatternIDs() {
		ids.Add(id)
	}
	return ids
}

// ruleID turns a tell's words into the name that selects it.
func ruleID(tell string) string {
	slug := strings.ToLower(tell)
	for _, article := range []string{"a ", "an ", "the "} {
		slug = strings.TrimPrefix(slug, article)
	}
	return "tombstones/" + strings.ReplaceAll(slug, " ", "-")
}

// Hit is a tombstone found in added text.
//
// Strippable means deleting LineNo removes this comment and nothing else.
// LineNo indexes Line in the text, and means nothing without Strippable.
type Hit struct {
	ID     string `json:"id"`     // the rule that fired, and the name that selects it
	Tell   string `json:"tell"`   // what that rule says in words
	Phrase string `json:"phrase"` // the matched words
	Line   string `json:"line"`   // the line they sit on

	Strippable bool `json:"strippable"`
	LineNo     int  `json:"lineNo"`
}

// Find returns the blocks over the cap. A non-positive maxLines turns it off.
//
// The wording rules are english.xml patterns, applied as a rewrite. Volume is
// here because it is a property of the block rather than of a phrase, and no
// rewording defeats it.
func Find(blocks []Block, maxLines int) []Hit {
	var hits []Hit
	for _, b := range blocks {
		if maxLines > 0 && b.Lines > maxLines {
			// A judgement about the whole block, not a span to excise, so
			// Strippable stays false.
			hits = append(hits, Hit{
				ID:     IDVolume,
				Tell:   "a comment block of " + strconv.Itoa(b.Lines) + " lines",
				Phrase: firstLine(b.Text),
				Line:   firstLine(b.Text),
				LineNo: -1,
			})
		}
	}
	return hits
}

// linePurity looks a block-relative line index up in b's parallel arrays.
func linePurity(b Block, li int) (lineNo int, pure bool) {
	if li < len(b.LineNos) && li < len(b.Pure) {
		return b.LineNos[li], b.Pure[li]
	}
	return -1, false
}

// HitForName builds the Hit for a dead referent, carrying the strip metadata a
// wording-based tell gets.
func HitForName(blocks []Block, name string) Hit {
	for _, b := range blocks {
		for li, line := range strings.Split(b.Text, "\n") {
			if !strings.Contains(line, name) {
				continue
			}
			lineNo, pure := linePurity(b, li)
			return Hit{
				ID:         ruleID(deadReferent),
				Tell:       deadReferent,
				Phrase:     name,
				Line:       strings.TrimSpace(line),
				Strippable: pure,
				LineNo:     lineNo,
			}
		}
	}
	return Hit{ID: ruleID(deadReferent), Tell: deadReferent, Phrase: name, Line: name, LineNo: -1}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
