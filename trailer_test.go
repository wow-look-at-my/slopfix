package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/markdown"
)

// commitMessage is a commit message with a git trailer block, as a hook sees
// it written to a file before `git commit -F`.
const commitMessage = "Read the runner from the visibility at run time\n" +
	"\n" +
	"The rule writes the expression and asks no network.\n" +
	"\n" +
	"Co-Authored-By: Example Author <author@example.com>\n" +
	"Claude-Session: https://claude.ai/code/session_01ExampleSessionIdentifier\n"

// Git reads each trailer line as a key and a value. No rule joins, rewrites
// or cuts one: the session trailer names no symbol of the repository.
func TestFixKeepsACommitMessagesTrailers(t *testing.T) {
	out := slopfix.Fix(slopfix.Request{Content: commitMessage, Path: "notes/commit.txt"})
	assert.Contains(t, out.Text, "\nCo-Authored-By: Example Author <author@example.com>\nClaude-Session: https://claude.ai/code/session_01ExampleSessionIdentifier\n", out.Text)
	assert.Empty(t, out.Removed)
}

// The trailer block is verbatim, and the paragraphs above it stay prose.
func TestTheTrailerBlockIsVerbatim(t *testing.T) {
	var prose []string
	for _, b := range markdown.Split(commitMessage) {
		if b.Kind == markdown.Prose {
			prose = append(prose, b.Text())
		}
	}
	assert.Equal(t, []string{"Read the runner from the visibility at run time", "The rule writes the expression and asks no network."}, prose)
}

// A last paragraph of prose with a colon is no trailer block.
func TestAClosingSentenceWithAColonStaysProse(t *testing.T) {
	doc := "The cache holds rows.\n\nNote: the gate reads every row.\n"
	var prose int
	for _, b := range markdown.Split(doc) {
		if b.Kind == markdown.Prose {
			prose++
		}
	}
	assert.Equal(t, 2, prose)
}
