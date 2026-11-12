package treecomments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// tmuxSize is the line a repair cut at the `#` inside its single quotes.
const tmuxSize = "\tsize=$(tmux display-message -p -t \"$TMUX_PANE\" '#{client_width} #{client_height}' 2>/dev/null) || size=\n"

// Tree-sitter loses the quotes after multiple here-documents on one line. Bash does not.
func TestAHashInsideQuotesAfterHereDocsIsNotAComment(t *testing.T) {
	src := "cat <<A <<B\nit's\nA\nit's\nB\n" + tmuxSize
	assert.Empty(t, texts(Extract("terminal.bash", src)))
}

func TestAHashThatBashDoesNotReadAsACommentIsNotOne(t *testing.T) {
	cases := map[string]string{
		"tmux line":          tmuxSize,
		"single quotes":      "echo 'a #b'\n",
		"double quotes":      "echo \"a #b\"\n",
		"ansi-c string":      "echo $'a #b'\n",
		"strip prefix":       "echo ${x#b}\n",
		"length":             "echo ${#x} b\n",
		"arithmetic base":    "echo $(( 16#ff )) b\n",
		"backslash escape":   "echo a \\#b\n",
		"here-document body": "cat <<EOF\n# b\nEOF\n",
		"two-line tmux form": "\t\tstate=$(tmux display-message -p -t \"$TMUX_PANE\" \\\n\t\t\t'#{pane_active}#{window_active}#{pane_in_mode}#{session_attached}' 2>/dev/null) || state=\n",
	}
	for name, src := range cases {
		assert.Empty(t, texts(Extract("x.bash", src)), name)
	}
}

func TestARealShellCommentIsStillFound(t *testing.T) {
	cases := map[string]struct{ src, want string }{
		"full line":             {"# a note\necho a\n", "# a note"},
		"after code":            {"echo a # a note\n", "# a note"},
		"after a quoted string": {"echo '#{a}' \"#b\" # a note\n", "# a note"},
	}
	for name, c := range cases {
		assert.Equal(t, []string{c.want}, texts(Extract("x.sh", c.src)), name)
	}
}

// A fragment bash cannot parse keeps only the comments that open their line.
func TestAnUnparsedShellFragmentKeepsWholeLineComments(t *testing.T) {
	src := "if true; then\n\t# a note\n\techo a # a tail\n"
	assert.Equal(t, []string{"# a note"}, texts(Extract("x.bash", src)))
}
