package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// The guard's repair of an Agent prompt is stated in rules/laziness-tool.xml.
// A test with out= runs with that prompt. A test with no out= is denied.
func TestEveryLazinessToolTestHolds(t *testing.T) {
	cases := table.MustLoad(rules.FS, "laziness-tool").Tests
	require.NotEmpty(t, cases)
	for _, c := range cases {
		t.Run(c.In, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Agent",
				"tool_input":      map[string]any{"prompt": c.In, "subagent_type": "Explore"},
			})
			require.NoError(t, err)
			res := laziness.Run(strings.NewReader(string(payload)), repairPunt)
			if c.Out == c.In {
				assert.Empty(t, res.Stdout, "a prompt with no tell runs as written")
				return
			}
			var out struct {
				HookSpecificOutput struct {
					PermissionDecision string         `json:"permissionDecision"`
					UpdatedInput       map[string]any `json:"updatedInput"`
				} `json:"hookSpecificOutput"`
			}
			require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out), res.Stdout)
			if c.Out == "" {
				assert.Equal(t, "deny", out.HookSpecificOutput.PermissionDecision)
				return
			}
			assert.Empty(t, out.HookSpecificOutput.PermissionDecision, "a repair is no refusal")
			assert.Equal(t, c.Out, out.HookSpecificOutput.UpdatedInput["prompt"])
			assert.Equal(t, "Explore", out.HookSpecificOutput.UpdatedInput["subagent_type"], "a field the guard does not read is kept")
		})
	}
}
