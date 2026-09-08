package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each launcher in the marketplace plugin, the flags it passes, and an input
// the rule behind it must answer.
//
// The sibling test pins that every name is registered and carries a summary. A
// registered subcommand that answers nothing is the failure this pins instead:
// the guard installs, runs, exits clean and does nothing, while every surface
// says it is enforcing. Reading silence as a dead guard costs a session.
// Reading it as fine costs the rule.
var launchers = []struct {
	// name is the subcommand a launcher execs.
	name string
	// only is what it passes to --only, or the empty string.
	only string
	// payload is stdin: a hook envelope the rule must act on.
	payload string
	// want is a fragment the answer has to carry.
	want string
}{{
	name: "hook", only: "counts",
	payload: writeEnvelope("docs/a.md", "It has three plugins.\n"),
	want:    `"content":"It has plugins.\n"`,
}, {
	name: "hook", only: "tombstones",
	payload: writeEnvelope("x.go", "package p\n\n// This used to read the flag from the environment.\nfunc f() {}\n"),
	want:    "used to read the flag",
}, {
	name: "message", only: "blame",
	payload: "The suite is red, but the failure is pre-existing.",
	want:    "blame/deflection",
}, {
	name: "message", only: "laziness/punt",
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
	payload: display("Fixed in PazerOP/foo#42."),
	want:    "](https://github.com/PazerOP/foo/",
}, {
	name:    "ask-properly",
	payload: display("I fixed the parser. Your call whether to ship it."),
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

// display builds a MessageDisplay payload for a finished message.
func display(delta string) string {
	return envelope(map[string]any{
		"hook_event_name": "MessageDisplay",
		"message_id":      "m",
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

// Every launcher answers the input its rule exists to catch.
func TestEveryLauncherAnswersItsOwnRule(t *testing.T) {
	for _, l := range launchers {
		t.Run(l.name+" "+l.only, func(t *testing.T) {
			c := find(t, l.name)
			if l.only != "" {
				require.NoError(t, c.Flags().Set("only", l.only))
				t.Cleanup(func() { _ = c.Flags().Set("only", "") })
			}
			var out bytes.Buffer
			c.SetIn(strings.NewReader(l.payload))
			c.SetOut(&out)
			c.SetErr(&bytes.Buffer{})
			_ = c.RunE(c, nil)

			require.NotEmpty(t, out.String(),
				"%s answered nothing: a guard that exits 0 in silence enforces nothing", l.name)
			assert.Contains(t, out.String(), l.want)
		})
	}
}
