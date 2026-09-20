// english.go applies the prose table the repair writes, and names the phrases
// it refuses to rewrite.
//
// The table itself is rules/, and nothing reads it at run time. cmd/rulegen
// parses that folder during the build. The generated file beside this holds
// every entry as a literal, with each pattern compiled to a switch automaton.
// Adding a word stays an edit to XML that needs no Go.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/table"
)

//go:generate go run github.com/wow-look-at-my/slopfix/cmd/rulegen -rules ../rules -for english -package commentfix -out english.gen.go

// english is the table the generated file carries.
var english = englishTable

// appliesTo reports where an entry applies. An empty where= means both, so an
// entry that says nothing about surface applies everywhere.
func appliesTo(where, surface string) bool {
	return table.AppliesTo(where, surface)
}

// Suggestion is prose the rule knows is bad and will not rewrite itself.
type Suggestion struct {
	// Phrase is what was found, as the table spells it.
	Phrase string `json:"phrase"`
	// Say names the repair, addressed to whoever reads the finding.
	Say string `json:"say"`
	// Line is where it sits, counting from the top of the file.
	Line int `json:"line"`
}

// Suggest reports the phrases in a file's comments that no rewrite can fix.
//
// It is separate from Check because it is separate advice: Check measures a
// comment against its code, and this reads what the comment says. A file can be
// clean by length and still carry every entry here.
func Suggest(filename, src string) []Suggestion {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	bs, ok := treeBlocks(language, src)
	if !ok {
		return nil
	}
	var out []Suggestion
	for _, b := range bs {
		for i, line := range b.text {
			lower := strings.ToLower(line)
			for _, f := range english.Flags {
				if !containsWord(lower, strings.ToLower(f.Phrase)) {
					continue
				}
				out = append(out, Suggestion{Phrase: f.Phrase, Say: f.Say, Line: b.start + i + 1})
			}
		}
	}
	return out
}

// containsWord reports a whole-word occurrence, so `hack` never matches
// `hackney` and `for now` never matches `for nowhere`.
func containsWord(s, word string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		if wordBoundary(s, at, at+len(word)) {
			return true
		}
		i = at + 1
	}
}
