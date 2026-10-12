package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
)

func TestCMakeCommentsAreTheLineCommentsCMakeReads(t *testing.T) {
	src := "# what the build needs\n" +
		"set(FLAGS\n" +
		"    \"-O3\"   # a note on the flag\n" +
		"    # a note above the next flag\n" +
		"    \"-g\"\n" +
		")\n" +
		"#[[ a bracket comment\n" +
		"# over two lines ]]\n" +
		"set(NOTE [=[\n" +
		"# data inside a bracket argument\n" +
		"]=])\n" +
		"set(QUOTED \"\n" +
		"# data inside a quoted argument\n" +
		"\")\n"

	var texts []string
	for _, c := range Extract("CMakeLists.txt", src) {
		texts = append(texts, c.Text)
	}
	assert.Equal(t, []string{"# what the build needs", "# a note above the next flag"}, texts)
}

// An edit that stays a comment to the bash fallback can still open a bracket
// comment, which swallows the code below it. The CMake gate refuses it.
func TestTheGateRefusesAnEditThatOpensABracketComment(t *testing.T) {
	src := "# a note\nset(X ON)\n# ]]\n"
	at := strings.Index(src, "a note")
	require.GreaterOrEqual(t, at, 0)

	res := Apply("CMakeLists.txt", src, []edit.Edit{{Start: at - 1, End: at - 1, Text: "[["}}, edit.Scope{})
	assert.Equal(t, src, res.Text)
	assert.Len(t, res.Refused, 1)

	// The same bytes in prose are a plain comment edit.
	res = Apply("CMakeLists.txt", src, []edit.Edit{{Start: at, End: at + len("a note"), Text: "the note"}}, edit.Scope{})
	assert.Equal(t, "# the note\nset(X ON)\n# ]]\n", res.Text)
}
