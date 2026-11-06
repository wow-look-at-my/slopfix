package agentlaunch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

func payload(t *testing.T, tool, input string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       tool,
		"tool_input":      json.RawMessage(input),
	})
	require.NoError(t, err)
	return string(data)
}

// The repairs are stated in rules/agent-launch.xml. A test with out= runs with
// that input. A test with no out= runs as written.
func TestEveryAgentLaunchTestHolds(t *testing.T) {
	t.Setenv(forceEnv, "")
	cases := table.MustLoad(rules.FS, "agent-launch").Tests
	require.NotEmpty(t, cases)
	for _, c := range cases {
		t.Run(c.In, func(t *testing.T) {
			res := Run(strings.NewReader(payload(t, "Agent", c.In)))
			if c.Out == "" {
				assert.Equal(t, Result{}, res)
				return
			}
			var out struct {
				HookSpecificOutput struct {
					PermissionDecision string          `json:"permissionDecision"`
					UpdatedInput       json.RawMessage `json:"updatedInput"`
				} `json:"hookSpecificOutput"`
			}
			require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out), res.Stdout)
			assert.Empty(t, out.HookSpecificOutput.PermissionDecision, "a repair is no refusal")
			assert.JSONEq(t, c.Out, string(out.HookSpecificOutput.UpdatedInput))
		})
	}
}

// With the model pinned, Claude Code drops `model` from the schema. The guard
// then moves the call to the background and adds no model.
func TestAPinnedModelIsLeftAlone(t *testing.T) {
	t.Setenv(forceEnv, "sonnet")
	res := Run(strings.NewReader(payload(t, "Agent", `{"prompt":"p","run_in_background":false,"subagent_type":"Explore"}`)))
	assert.Contains(t, res.Stdout, `"run_in_background":true`)
	assert.NotContains(t, res.Stdout, `"model"`)
}

func TestOtherToolsAndEventsAreNotJudged(t *testing.T) {
	input := `{"run_in_background":false,"subagent_type":"Explore"}`
	assert.Equal(t, Result{}, Run(strings.NewReader(payload(t, "Bash", input))))
	stop := strings.Replace(payload(t, "Agent", input), "PreToolUse", "Stop", 1)
	assert.Equal(t, Result{}, Run(strings.NewReader(stop)))
	assert.Equal(t, Result{}, Run(strings.NewReader("not json")))
}
