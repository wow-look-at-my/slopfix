package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// What the tidy pass writes for a whole line is stated in english.xml and
// driven by the english package's own test. This is the property behind those
// cases, which no worked example states: the repair may drop a word and may
// rewrite a word, and may never run two of the author's words together.
func TestTighteningNeverWeldsTwoWordsTogether(t *testing.T) {
	for _, in := range []string{
		"It reads from the .gitmodules parser above.",
		"Note that it is basically just the .config directory it actually reads.",
		"In order to bind the socket it obviously needs the .env file at startup.",
		"The walk skips .git , .gitmodules and .gitignore alike.",
	} {
		t.Run(in, func(t *testing.T) {
			assert.Empty(t, welded(in, shorten(in)))
			assert.Empty(t, welded(in, Deslop(in)))
		})
	}
}

// welded names an output word made of the input carried side by side, and is
// empty when the tidy pass kept them apart.
func welded(in, out string) string {
	have := make(map[string]bool)
	for _, word := range strings.Fields(in) {
		have[word] = true
	}
	for _, word := range strings.Fields(out) {
		if have[word] {
			continue
		}
		for cut := 1; cut < len(word); cut++ {
			// A mark the source left stranded is a single the tidy pass is
			// meant to pull back onto the word in front of it.
			if allMarks(word[:cut]) || allMarks(word[cut:]) {
				continue
			}
			if have[word[:cut]] && have[word[cut:]] {
				return word
			}
		}
	}
	return ""
}

// allMarks reports a run of punctuation carrying no word in it.
func allMarks(s string) bool {
	return strings.Trim(s, ",.;:!?") == ""
}
