package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
)

// Each input is prose the write hook once repaired into false or broken text.
// The repair of each must read as grammatical, true English.
func TestARepairWritesTrueText(t *testing.T) {
	for _, c := range []struct {
		name, in, out string
	}{
		{
			name: "a by-phrase of another participle names no actor",
			in:   "Every repository the session touches is recorded in a per-session state file keyed by `session_id`, the first time it is touched:",
			out:  "Every repository the session touches is recorded in a per-session state file keyed by `session_id`, the first time it is touched:",
		},
		{
			name: "a verb that ends in a silent e keeps it",
			in:   "Unpushed commits, `stash push` and `worktree add` are judged by Found work below.",
			out:  "Found work below judges unpushed commits, `stash push` and `worktree add`.",
		},
		{
			name: "a plural actor takes a plural verb, and only stays with the actor",
			in:   "Work inside a worktree, and `git worktree remove` of a clean worktree, are judged only by the rules above for that worktree.",
			out:  "Only the rules above for that worktree judge work inside a worktree, and `git worktree remove` of a clean worktree.",
		},
		{
			name: "between takes a pair",
			in:   "An element is the phrase between two joiners.",
			out:  "An element is the phrase between a pair of joiners.",
		},
		{
			name: "a relative clause after a preposition keeps its verb",
			in:   "A fenced block in which `MinMembers` or more lines that are not comments share a leading word and differ in the word after it, which is a usage block that walks through the subcommands of a program.",
			out:  "A fenced block. In that block, `MinMembers` or more lines that are not comments share a leading word and differ in the word after it. This is a usage block that walks through the subcommands of a program.",
		},
		{
			name: "a coordinated subject is named again as they",
			in:   "An index, a table of contents, a rule table, a command table, a word list and a spec page are all enumerations and are all reported.",
			out:  "An index, a table of contents, a rule table, a command table, a word list and a spec page are all enumerations. They are all reported.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := slopfix.Fix(slopfix.Request{Content: c.in + "\n", Path: "x.md"}).Text
			assert.Equal(t, c.out, strings.TrimSpace(got))
		})
	}
}
