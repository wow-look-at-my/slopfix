package autoallow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Allowed answers for one simple command, from the same table the hook reads.
func TestAllowedJudgesOneCommandAgainstTheShippedTable(t *testing.T) {
	for _, argv := range [][]string{
		{"git", "status", "-sb"},
		{"git", "-C", "spec", "log", "--oneline", "-3"},
		{"git", "submodule", "status"},
		{"head", "-5", "f"},
		{"find", ".", "-name", "*.h", "-exec", "grep", "-l", "X", "{}", ";"},
	} {
		assert.True(t, Allowed(argv), "%q only reads", argv)
	}
	for _, argv := range [][]string{
		{"git", "push"},
		{"fd", "-x", "rm"},
		{"sort", "-uo", "out", "f"},
		{"python3", "x.py"},
		{"node", "-e", "1"},
		{"find", ".", "-exec", "rm", "{}", ";"},
		{},
	} {
		assert.False(t, Allowed(argv), "%q is not allowed", argv)
	}
}

// allowedFlags="*" admits any flag, in any spelling, and any operand.
func TestARuleThatAllowsEveryFlagAllowsEverySpelling(t *testing.T) {
	for _, argv := range [][]string{
		{"ls", "-la", "--color=auto", "--group-directories-first", "a", "b"},
		{"cat", "-nA", "f"},
		{"grep", "-rniE", "--include=*.go", "foo|bar", "."},
		{"git", "status", "--porcelain=v2", "-uno", "--branch"},
		{"wc", "-l"},
	} {
		assert.True(t, Allowed(argv), "%q", argv)
	}
}

// An argument nobody can read passes only a rule that admits every argument.
func TestAnUnknownWordPassesOnlyARuleThatAdmitsEveryArgument(t *testing.T) {
	for _, argv := range [][]string{
		{"cat", UnknownWord},
		{"ls", "-la", UnknownWord},
		{"git", "status", UnknownWord},
		{"git", "-C", UnknownWord, "status"},
	} {
		assert.True(t, Allowed(argv), "%q", argv)
	}
	for _, argv := range [][]string{
		// The word could be --output=FILE, which the rule denies.
		{"git", "log", UnknownWord},
		// The word stands where a subcommand goes.
		{"git", UnknownWord},
		// The word could be --delete, which a deny rule of git push refuses.
		{"git", "push", UnknownWord},
		// find runs commands through -exec, and fd through -x.
		{"find", ".", "-name", UnknownWord},
		{"fd", UnknownWord},
		// The word could be a second operand, which uniq writes.
		{"uniq", UnknownWord},
		{"sort", UnknownWord},
	} {
		assert.False(t, Allowed(argv), "%q", argv)
	}
}

// A wholeCommand rule judges a whole command line, so it does not refuse one statement of it.
func TestAllowedIgnoresTheRulesThatJudgeAWholeCommandLine(t *testing.T) {
	assert.True(t, Allowed([]string{"true"}))
}

func TestAllowedMCPReadsTheShippedServers(t *testing.T) {
	assert.True(t, AllowedMCP("github", "get_file_contents"))
	assert.False(t, AllowedMCP("github", "merge_pull_request"))
	assert.False(t, AllowedMCP("no-such-server", "get_thing"))
}
