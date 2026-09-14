package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The tidy pass closed the gap in front of every period it found, and a period
// with a word against its right side opens a file name rather than closing a
// sentence. Comments shipped reading `from the.gitmodules parser`.
//
// The gap only closes when the period closes something, which is what the
// space after it says.
func TestTighteningKeepsTheSpaceInFrontOfAFileName(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{
			name: "a dotfile after a determiner",
			in:   "It reads from the .gitmodules parser above.",
			want: "It reads from the .gitmodules parser above.",
		},
		{
			name: "a dotfile mid sentence",
			in:   "The walk skips .git and reads .gitmodules instead.",
			want: "The walk skips .git and reads .gitmodules instead.",
		},
		{
			name: "a gap a deletion really left",
			in:   "It reads the file .",
			want: "It reads the file.",
		},
		{
			name: "a gap in front of a comma",
			in:   "It reads the file , and stops.",
			want: "It reads the file, and stops.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, shorten(c.in))
		})
	}
}

// The property behind the case above. A tidy pass may drop a word and may
// rewrite one, and it may never run two of the author's words into one.
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

// welded names an output word made of two the input carried side by side, and
// is empty when the tidy pass kept them apart.
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
			// A mark the source left stranded is one the tidy pass is meant to
			// pull back onto the word in front of it.
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
