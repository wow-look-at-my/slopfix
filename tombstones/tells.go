// tells.go holds what a tombstone looks like on the page.
//
// A property separates it from a comment worth keeping. Its referent is gone:
// the flag, the test, the spelling it names does not exist any more. Or its
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

// deadReferent is the tell referents.go reports, and IDDeadReferent the rule that reports it.
const deadReferent = "a name nothing in the repository defines"

// IDDeadReferent names the rule a dead referent reports under.
var IDDeadReferent = ruleID(deadReferent)

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
type Hit struct {
	ID     string `json:"id"`     // the rule that fired, and the name that selects it
	Tell   string `json:"tell"`   // what that rule says in words
	Phrase string `json:"phrase"` // the matched words
	Line   string `json:"line"`

	Strippable bool `json:"strippable"`
	// LineNo is the line of the hit, counted from one.
	LineNo int `json:"lineNo"`
	// EndLineNo is the last line of a hit that judges a whole block, counted from one, and zero for any other hit.
	EndLineNo int `json:"endLineNo,omitempty"`
	// Fix asks for a rewrite by hand when no repair can make the text whole. It is empty for any other hit.
	Fix string `json:"fix,omitempty"`
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
			// Strippable stays false. The hit names the lines the block covers.
			hit := Hit{
				ID:     IDVolume,
				Tell:   "a comment block of " + strconv.Itoa(b.Lines) + " lines",
				Phrase: firstLine(b.Text),
				Line:   firstLine(b.Text),
				LineNo: -1,
			}
			if len(b.LineNos) > 0 {
				hit.LineNo, hit.EndLineNo = b.LineNos[0]+1, b.LineNos[len(b.LineNos)-1]+1
			}
			hits = append(hits, hit)
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
		lines := strings.Split(b.Text, "\n")
		// The name is sought in the comment prose, so a code line that uses it never places the hit.
		for li, prose := range strings.Split(b.Prose, "\n") {
			if !strings.Contains(prose, name) || li >= len(lines) {
				continue
			}
			line := lines[li]
			row, pure := linePurity(b, li)
			return Hit{
				ID:         ruleID(deadReferent),
				Tell:       deadReferent,
				Phrase:     name,
				Line:       strings.TrimSpace(line),
				Strippable: pure,
				LineNo:     row + 1,
			}
		}
	}
	return Hit{ID: ruleID(deadReferent), Tell: deadReferent, Phrase: name, Line: name}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
