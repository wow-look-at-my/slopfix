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

// A wholeCommand rule judges a whole command line, so it does not refuse one statement of it.
func TestAllowedIgnoresTheRulesThatJudgeAWholeCommandLine(t *testing.T) {
	assert.True(t, Allowed([]string{"true"}))
}

func TestAllowedMCPReadsTheShippedServers(t *testing.T) {
	assert.True(t, AllowedMCP("github", "get_file_contents"))
	assert.False(t, AllowedMCP("github", "merge_pull_request"))
	assert.False(t, AllowedMCP("no-such-server", "get_thing"))
}
