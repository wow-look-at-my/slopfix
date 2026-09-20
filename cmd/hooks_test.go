package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A subcommand silent on a payload it should refuse guards nothing.

// Subcommands are shared, and driving them writes their streams and flags.
var cmdMu sync.Mutex

// find returns the registered subcommand of that name.
func find(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("no subcommand named %q is registered", name)
	return nil
}

// run drives a subcommand with a payload on stdin and returns its stdout.
func run(t *testing.T, name, payload string) string {
	t.Helper()
	cmdMu.Lock()
	defer cmdMu.Unlock()
	c := find(t, name)
	var out bytes.Buffer
	c.SetIn(strings.NewReader(payload))
	c.SetOut(&out)
	c.SetErr(&bytes.Buffer{})
	require.NoError(t, c.RunE(c, nil))
	return out.String()
}

// Every launcher in the marketplace plugin execs a name below. The names are
// the contract between both repositories, so they are pinned here.
func TestEveryHookSubcommandIsRegistered(t *testing.T) {
	for _, name := range []string{
		"ask-properly", "command", "file", "hook", "link-refs", "md-budget",
	} {
		t.Run(name, func(t *testing.T) {
			c := find(t, name)
			assert.NotEmpty(t, c.Short, "a subcommand with no summary is one nobody can discover")
		})
	}
}

// An event a subcommand does not serve leaves the call alone. Printing there
// is how a hook interferes with work it was never meant to judge.
func TestAnUnservedEventIsSilent(t *testing.T) {
	payload := `{"hook_event_name":"SessionEnd","tool_name":"Bash","tool_input":{"command":"ls"}}`
	assert.Equal(t, hookResult{}, judgeCommand([]byte(payload)))
	for _, name := range []string{"ask-properly", "link-refs"} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, run(t, name, payload))
		})
	}
}

// A payload that does not parse leaves the call alone too. A guard that refuses
// what it could not read is worse than no guard.
func TestAnUnreadablePayloadIsSilent(t *testing.T) {
	assert.Equal(t, hookResult{}, judgeCommand([]byte("{ not json")))
	for _, name := range []string{"ask-properly", "link-refs", "md-budget"} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, run(t, name, "{ not json"))
		})
	}
}

// The refusal the Bash cleaner owes on a heredoc, reached through the thing
// command that now carries it. The write tools are what the message has to name.
func TestCommandCheckRefusesAHeredoc(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "cat <<EOF\nhello\nEOF\n"},
	})
	require.NoError(t, err)

	res := judgeCommand(payload)
	require.NotEmpty(t, res.Stdout, "a heredoc must be refused")

	var resp struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &resp))
	assert.Equal(t, "PreToolUse", resp.HookSpecificOutput.HookEventName)
	assert.Equal(t, "deny", resp.HookSpecificOutput.PermissionDecision)
	assert.NotEmpty(t, resp.HookSpecificOutput.PermissionDecisionReason)
}

// The control that proves the case above can fail: an ordinary command carries
// no heredoc and is not refused.
func TestCommandCheckLeavesAnOrdinaryCommandAlone(t *testing.T) {
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}`
	assert.NotContains(t, judgeCommand([]byte(payload)).Stdout, `"permissionDecision":"deny"`)
}

// A tool that writes no file is none of the Bash cleaner's business.
func TestCommandCheckIgnoresAnotherTool(t *testing.T) {
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"x.go"}}`
	assert.Empty(t, judgeCommand([]byte(payload)).Stdout)
}

// A guard that fires is named, so a refusal points at the package that owes it.
func TestAFiredGuardIsNamedOnStderr(t *testing.T) {
	t.Serial()
	fired := guard{"pretend", func(io.Reader) hookResult {
		return hookResult{Stderr: "refused\n", Code: 2}
	}}
	restore := guards
	guards = []guard{fired}
	t.Cleanup(func() { guards = restore })

	res := judgeCommand([]byte(`{"hook_event_name":"PreToolUse"}`))

	assert.Equal(t, 2, res.Code)
	assert.Equal(t, "slopfix command pretend: refused\n", res.Stderr)
}

// The order is the rule: a rewrite or a refusal answers ahead of an approval.
func TestTheFirstGuardWithAResponseAnswers(t *testing.T) {
	t.Serial()
	restore := guards
	guards = []guard{
		{"silent", func(io.Reader) hookResult { return hookResult{} }},
		{"speaks", func(io.Reader) hookResult { return hookResult{Stdout: "first"} }},
		{"later", func(io.Reader) hookResult { return hookResult{Stdout: "second"} }},
	}
	t.Cleanup(func() { guards = restore })

	assert.Equal(t, "first", judgeCommand([]byte(`{}`)).Stdout)
}
