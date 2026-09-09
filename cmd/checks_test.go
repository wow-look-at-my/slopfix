package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each check a marketplace launcher reaches, the checks it selects, and an
// input that check must answer.
//
// The sibling test pins that every name is registered and carries a summary. A
// registered subcommand that answers nothing is the failure this pins instead:
// the guard installs, runs, exits clean and does nothing, while every surface
// says it is enforcing. Reading silence as a dead guard costs a session.
// Reading it as fine costs the rule.
var checks = []struct {
	// name is the subcommand.
	name string
	// only is the check list --only selects, or none.
	only []string
	// payload is stdin: a hook envelope the check must act on.
	payload string
	// want is a fragment the answer has to carry.
	want string
}{{
	name: "hook", only: []string{"counts"},
	payload: writeEnvelope("docs/a.md", "It has three plugins.\n"),
	want:    `"content":"It has plugins.\n"`,
}, {
	name: "hook", only: []string{"tombstones"},
	payload: writeEnvelope("x.go", "package p\n\n// This used to read the flag from the environment.\nfunc f() {}\n"),
	want:    `"content":"package p\n\nfunc f() {}\n"`,
}, {
	name: "message", only: []string{"blame"},
	payload: "The suite is red, but the failure is pre-existing.",
	want:    "blame/deflection",
}, {
	name: "message", only: []string{"laziness/punt"},
	payload: "I found an off-by-one in the retry loop and left it alone.",
	want:    "laziness/punt",
}, {
	name:    "auto-allow",
	payload: bash("python3 -c 1"),
	want:    `"permissionDecision":"deny"`,
}, {
	name:    "clean-bash",
	payload: bash("docker compose restart web"),
	want:    "up -d",
}, {
	name:    "no-work-loss",
	payload: bash("sed -i s/a/b/ README.md"),
	want:    `"permissionDecision":"deny"`,
}, {
	name:    "link-refs",
	payload: display("link-refs", "Fixed in PazerOP/foo#42."),
	want:    "](https://github.com/PazerOP/foo/",
}, {
	name:    "ask-properly",
	payload: display("ask-properly", "I fixed the parser. Your call whether to ship it."),
	want:    "ask-properly",
}}

// writeEnvelope builds a PreToolUse payload for a Write of a file.
func writeEnvelope(path, content string) string {
	return envelope(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": path, "content": content},
	})
}

// bash builds a PreToolUse payload for a shell command.
func bash(command string) string {
	return envelope(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
	})
}

// display builds a MessageDisplay payload for a finished message. A check
// keys its per-message state by the id, so each case brings its own.
func display(id, delta string) string {
	return envelope(map[string]any{
		"hook_event_name": "MessageDisplay",
		"message_id":      id,
		"final":           true,
		"delta":           delta,
	})
}

func envelope(payload map[string]any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// selectChecks points --only at exactly this list. Set APPENDS to a string
// slice after the flag has been touched, so a later case would run its
// predecessor's checks too, and pass on whichever answered.
func selectChecks(t *testing.T, c *cobra.Command, only []string) {
	t.Helper()
	value, ok := c.Flags().Lookup("only").Value.(pflag.SliceValue)
	require.True(t, ok, "--only must take a check list")
	require.NoError(t, value.Replace(only))
	t.Cleanup(func() { _ = value.Replace(nil) })
}

// Every check answers the input it exists to catch.
func TestEveryCheckAnswersItsOwnInput(t *testing.T) {
	for _, check := range checks {
		t.Run(check.name+" "+strings.Join(check.only, ","), func(t *testing.T) {
			cmdMu.Lock()
			defer cmdMu.Unlock()
			c := find(t, check.name)
			if len(check.only) > 0 {
				selectChecks(t, c, check.only)
			}
			var out bytes.Buffer
			c.SetIn(strings.NewReader(check.payload))
			c.SetOut(&out)
			c.SetErr(&bytes.Buffer{})
			_ = c.RunE(c, nil)

			require.NotEmpty(t, out.String(),
				"%s answered nothing: a guard that exits clean in silence enforces nothing", check.name)
			assert.Contains(t, out.String(), check.want)
		})
	}
}
