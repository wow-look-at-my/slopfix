package laziness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolCall(t *testing.T, tool string, input map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       tool,
		"tool_input":      input,
	})
	require.NoError(t, err)
	return string(data)
}

func denyReason(t *testing.T, res Result) string {
	t.Helper()
	if res.Stdout == "" {
		return ""
	}
	var out struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	require.Equal(t, "deny", out.HookSpecificOutput.PermissionDecision)
	return out.HookSpecificOutput.PermissionDecisionReason
}

func question(text string, options ...[2]string) map[string]any {
	var opts []any
	for _, o := range options {
		opts = append(opts, map[string]any{"label": o[0], "description": o[1]})
	}
	return map[string]any{"questions": []any{map[string]any{
		"question": text, "header": "h", "multiSelect": false, "options": opts,
	}}}
}

func TestAQuestionAskingPermissionToFixIsRefused(t *testing.T) {
	in := question(
		"Can I fix the submodule checkout race in wow-look-at-my/actions? A saved memory says repos other than gosmopolitan and go-toolchain are off limits unless you name them.",
		[2]string{"Yes, fix it (Recommended)", "Branch in wow-look-at-my/actions."},
		[2]string{"No, leave it", "slopfix PR 281 stays red."},
	)
	reason := denyReason(t, run(t, toolCall(t, "AskUserQuestion", in)))
	assert.Contains(t, reason, ID)
	assert.Contains(t, reason, "Can I fix the submodule checkout race")
	assert.NotContains(t, reason, "\n")
}

func TestAQuestionBetweenTwoDesignsIsAllowed(t *testing.T) {
	in := question(
		"Which store should the walk cache use?",
		[2]string{"Per-directory mtimes", "One stat per directory on a warm run."},
		[2]string{"FSEvents journal", "Replays changes since the last event id."},
	)
	assert.Equal(t, Result{}, run(t, toolCall(t, "AskUserQuestion", in)))
}

func TestAnAgentPromptThatGrantsAFixIsRefused(t *testing.T) {
	for _, prompt := range []string{
		"The user authorized fixing wow-look-at-my/actions. Task: fix the race in parallel-checkout.",
		"Work in go-toolchain only. Other repos are off limits unless the user names it.",
		"Fix the walk. Do not touch wow-look-at-my/actions; I am asking the user about it.",
	} {
		assert.NotEmpty(t, denyReason(t, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt, "description": "d"}))), prompt)
	}
}

func TestAnAgentPromptNamingARealBlockerIsAllowed(t *testing.T) {
	prompt := "Fix the red build. go-toolchain master c021e219 never published because its host-build job failed checking out _gosmopolitan; fix that race in wow-look-at-my/actions first."
	assert.Equal(t, Result{}, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt, "description": "d"})))
}

func TestASendMessageAskingPermissionIsRefused(t *testing.T) {
	in := map[string]any{"to": "a", "message": "Do not touch the actions repo; I'm asking the user about it."}
	assert.NotEmpty(t, denyReason(t, run(t, toolCall(t, "SendMessage", in))))
}

func TestAStructuredSendMessageIsRead(t *testing.T) {
	in := map[string]any{"to": "a", "message": map[string]any{"type": "note", "body": "This needs the user's approval before you fix it."}}
	assert.NotEmpty(t, denyReason(t, run(t, toolCall(t, "SendMessage", in))))
}

func TestOtherToolsAreNotJudged(t *testing.T) {
	assert.Equal(t, Result{}, run(t, toolCall(t, "Bash", map[string]any{"command": "echo 'Can I fix it?'"})))
}

func TestAnUnreadableToolInputIsAllowed(t *testing.T) {
	for _, body := range []string{
		`{"hook_event_name":"PreToolUse","tool_name":"Agent","tool_input":"nope"}`,
		`{"hook_event_name":"PreToolUse","tool_name":"AskUserQuestion"}`,
		`{"hook_event_name":"PreToolUse","tool_name":"SendMessage","tool_input":{"message":7}}`,
	} {
		assert.Equal(t, Result{}, Run(strings.NewReader(body)), body)
	}
}
