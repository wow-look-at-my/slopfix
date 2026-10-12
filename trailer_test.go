package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
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

// Each line that opens with a field is a prose block of its own. Every
// rule reads its value and no reflow joins it to the next.
func TestEachTrailerLineIsProseOfItsOwn(t *testing.T) {
	var prose []string
	for _, b := range markdown.Split(commitMessage) {
		if b.Kind == markdown.Prose {
			prose = append(prose, b.Text())
		}
	}
	assert.Equal(t, []string{
		"Read the runner from the visibility at run time",
		"The rule writes the expression and asks no network.",
		"Co-Authored-By: Example Author <author@example.com>",
		"Claude-Session: https://claude.ai/code/session_01ExampleSessionIdentifier",
	}, prose)
}

// A sentence over the cap in a trailer value is found and repaired like any
// other, and the trailer lines stay apart.
func TestALongSentenceInATrailerValueIsRepaired(t *testing.T) {
	msg := "Read the runner at run time\n" +
		"\n" +
		"The rule asks no network.\n" +
		"\n" +
		"Reviewed-Note: This sentence carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n" +
		"Co-Authored-By: Example Author <author@example.com>\n"
	found := false
	for _, f := range slopfix.CheckContent("notes/commit.txt", msg) {
		found = found || f.ID == ste.IDSentenceCap
	}
	assert.True(t, found, "the long sentence in the trailer value was not found")
	out := slopfix.Fix(slopfix.Request{Content: msg, Path: "notes/commit.txt"}).Text
	for _, f := range slopfix.CheckContent("notes/commit.txt", out) {
		assert.NotEqual(t, ste.IDSentenceCap, f.ID, "%s\n%s", f.Detail, out)
	}
	assert.Contains(t, out, "\nCo-Authored-By: Example Author <author@example.com>\n", out)
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
