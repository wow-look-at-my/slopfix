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
	"github.com/wow-look-at-my/go-containers/set"
	"regexp"
	"strconv"
	"strings"
)

// IDVolume names the volume cap, whose tell carries the block's own size and so
// cannot supply a stable name of its own.
const IDVolume = "tombstones/comment-volume"

// ruleID turns a tell's words into the name that selects it. The table stays
// data: a new row needs no second entry anywhere.
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

// tell is a recognisable shape. name is what a report prints.
type tell struct {
	name string
	re   *regexp.Regexp
}

// changeParticiples are the verbs that describe an edit rather than a state.
// A comment reaches for one of these only to narrate what a commit did.
const changeParticiples = `renamed|removed|added|deleted|moved|replaced|` +
	`introduced|dropped|split|merged|reverted|refactored|extracted|migrated|` +
	`deprecated|rewritten|rewrote|bumped|reworked|consolidated|inlined|hoisted`

// tells is the table. It is data: extending this package is adding a row.
//
// Every row traces to a shape that survives paraphrase badly. The tiers that do
// not read the wording at all are the volume cap in Find and referents.go.
var tells = []tell{
	// Git already timestamps the line, so a date can only narrate.
	{"a date", regexp.MustCompile(`(?i)\b(?:19|20)\d{2}-\d{2}-\d{2}\b`)},
	{"a date", regexp.MustCompile(`(?i)\b(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?\s+(?:\d{1,2},?\s+)?(?:19|20)\d{2}\b`)},

	// The commit message is where a change reference belongs.
	{"a change reference", regexp.MustCompile(`(?i)\b(?:pr|pull request|issue|ticket|commit)\s+#?\d+\b`)},
	{"a change reference", regexp.MustCompile(`(?i)\b[a-z0-9][\w.-]*/[\w.-]+#\d+\b`)},

	// A contrast marker states a "then" the reader cannot see.
	{"a then-and-now contrast", regexp.MustCompile(`(?i)\brather than (?:the )?(?:old|former|previous|legacy)\b`)},
	{"a then-and-now contrast", regexp.MustCompile(`(?i)\binstead of (?:the )?(?:old|former|previous|legacy)\b`)},
	{"a then-and-now contrast", regexp.MustCompile(`(?i)\bwhere (?:it|this|that) (?:used to|once)\b`)},
	{"a then-and-now contrast", regexp.MustCompile(`(?i)\b[a-z]+ed now\b|\b(?:is|are) now (?:[a-z]+ed|the case)\b`)},

	// The referent is gone: the sentence's subject is a former state.
	{"a former state", regexp.MustCompile(`(?i)\bused to\b`)},
	{"a former state", regexp.MustCompile(`(?i)\b(?:previously|formerly|originally|hitherto)\b`)},
	{"a former state", regexp.MustCompile(`(?i)\bno longer\b|\banymore\b|\bnowadays\b|\bthese days\b`)},
	{"a former state", regexp.MustCompile(`(?i)\bthe (?:former|old|previous|legacy|original) \w+`)},
	{"a former state", regexp.MustCompile(`(?i)\b(?:was|were|has been|have been|had been|got|gets|is now|are now) (?:` + changeParticiples + `)\b`)},
	// A demonstrative in front of a participle needs the narrower verb set,
	// because "that split has a way to go wrong" is a noun.
	{"a former state", regexp.MustCompile(`(?i)\b(?:we|this|it|that) (?:renamed|removed|deleted|replaced|introduced|reverted|refactored|migrated|deprecated|rewrote|reworked|consolidated)\b`)},
	{"a former state", regexp.MustCompile(`(?i)\bthis (?:replaces|supersedes|used to)\b`)},
	{"a former state", regexp.MustCompile(`(?i)\bstopped (?:being|working|doing)\b|\bstarted (?:being|failing)\b`)},

	// The audience is the reviewer, not the next editor.
	{"an address to the reviewer", regexp.MustCompile(`(?i)\bthis (?:pr|pull request|change|diff|commit|patch|cl)\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\bworth (?:noting|your attention|knowing here)\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\bwhat is worth [a-z]+ing here\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\b(?:as|when) requested\b|\bwas never requested\b|\bnobody asked\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\bdo not (?:reintroduce|add this back|bring (?:it|this) back)\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\bper the (?:review|reviewer|feedback|comment)\b`)},
	{"an address to the reviewer", regexp.MustCompile(`(?i)\b(?:flagging|to be clear|just to note|for the reviewer)\b`)},

	// An argument for the diff, parked permanently in a file.
	{"a defence of the change", regexp.MustCompile(`(?i)\bthat is not (?:tidying|cleanup|cosmetic|churn|a rename|style|gratuitous)\b`)},
	{"a defence of the change", regexp.MustCompile(`(?i)\bthis is not (?:just )?(?:tidying|cleanup|cosmetic|churn|a rename|refactoring for)\b`)},
	{"a defence of the change", regexp.MustCompile(`(?i)\bwould have been\b|\bwould otherwise have\b`)},
	{"a defence of the change", regexp.MustCompile(`(?i)\bnot (?:tidying|scope creep|gold.?plating)\b`)},

	// An instruction, quoted. It reads as authority the code cannot check.
	{"a quoted instruction", regexp.MustCompile(`(?i)\bthe (?:owner|operator|user|reviewer|maintainer) (?:said|says|ruled|asked|wants|requested)\b`)},
	{"a quoted instruction", regexp.MustCompile(`(?i)\bper (?:the )?(?:owner|operator|user|maintainer)\b`)},
	{"a quoted instruction", regexp.MustCompile(`(?i)\bby (?:owner|operator) (?:ruling|request|decree)\b`)},

	// An experiment reported to the reviewer, which nothing re-runs.
	{"a report of an experiment", regexp.MustCompile(`(?i)\bnegative control\b`)},
	{"a report of an experiment", regexp.MustCompile(`(?i)\b(?:i|we) (?:ran|tried|tested|measured|verified this by|checked this by)\b`)},
	{"a report of an experiment", regexp.MustCompile(`(?i)\bverified by (?:breaking|deleting|removing|reverting)\b`)},
	{"a report of an experiment", regexp.MustCompile(`(?i)\brun before trusting\b`)},
}

// Find returns every tombstone the blocks carry.
//
// maxLines caps a block, and zero turns the cap off. A tombstone is surplus
// text, so volume catches the essay no rewording defeats.
func Find(blocks []Block, maxLines int) []Hit {
	var hits []Hit
	seen := set.New[string]()
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
		for li, line := range strings.Split(b.Text, "\n") {
			for _, t := range tells {
				at := t.re.FindStringIndex(line)
				if at == nil {
					continue
				}
				phrase := strings.TrimSpace(line[at[0]:at[1]])
				// A line reports each tell a single time, because
				// printing every row that matched is noise.
				key := t.name + "\x00" + strings.TrimSpace(line)
				if seen.Contains(key) {
					continue
				}
				seen.Add(key)
				h := Hit{ID: ruleID(t.name), Tell: t.name, Phrase: phrase, Line: strings.TrimSpace(line), LineNo: -1}
				h.LineNo, h.Strippable = linePurity(b, li)
				hits = append(hits, h)
			}
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
				ID:         ruleID("a name nothing in the repository defines"),
				Tell:       "a name nothing in the repository defines",
				Phrase:     name,
				Line:       strings.TrimSpace(line),
				Strippable: pure,
				LineNo:     lineNo,
			}
		}
	}
	dead := "a name nothing in the repository defines"
	return Hit{ID: ruleID(dead), Tell: dead, Phrase: name, Line: name, LineNo: -1}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
