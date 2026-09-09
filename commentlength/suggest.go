// suggest.go reports the phrases the table refuses to rewrite. It lives here
// rather than beside the table because it needs the parse to find the comments.
package commentlength

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/english"
)

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
			for _, f := range english.Flags() {
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
