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

// cutPunts stands in for the `fix --message` cut, which lives in the root
// package: it deletes each sentence that Check reports.
func cutPunts(text string) string {
	for _, hit := range Check(text) {
		text = strings.Replace(text, hit.Sentence, "", 1)
	}
	return strings.TrimSpace(text)
}

// hookOutput is what the guard prints on PreToolUse.
type hookOutput struct {
	HookSpecificOutput struct {
		PermissionDecision       string         `json:"permissionDecision"`
		PermissionDecisionReason string         `json:"permissionDecisionReason"`
		UpdatedInput             map[string]any `json:"updatedInput"`
	} `json:"hookSpecificOutput"`
}

func decode(t *testing.T, res Result) hookOutput {
	t.Helper()
	var out hookOutput
	require.NotEmpty(t, res.Stdout)
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	return out
}

func denyReason(t *testing.T, res Result) string {
	t.Helper()
	out := decode(t, res)
	require.Equal(t, "deny", out.HookSpecificOutput.PermissionDecision)
	return out.HookSpecificOutput.PermissionDecisionReason
}

// repaired answers the input the guard lets the call run with. A repair sets
// no permission decision, so the normal permission flow still judges the call.
func repaired(t *testing.T, res Result) map[string]any {
	t.Helper()
	out := decode(t, res)
	require.Empty(t, out.HookSpecificOutput.PermissionDecision)
	require.NotNil(t, out.HookSpecificOutput.UpdatedInput)
	return out.HookSpecificOutput.UpdatedInput
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

func TestAnAgentPromptThatGrantsAFixIsRepaired(t *testing.T) {
	for prompt, want := range map[string]string{
		"The user authorized fixing wow-look-at-my/actions. Task: fix the race in parallel-checkout.": "Task: fix the race in parallel-checkout.",
		"Work in go-toolchain only. Other repos are off limits unless the user names it.":             "Work in go-toolchain only.",
	} {
		in := repaired(t, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt, "description": "d"})))
		assert.Equal(t, want, in["prompt"], prompt)
		assert.Equal(t, "d", in["description"], "a field the guard does not read is kept")
	}
}

func TestAPromptWithNothingLeftAfterTheRepairIsDenied(t *testing.T) {
	prompt := "The user authorized fixing wow-look-at-my/actions."
	reason := denyReason(t, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt})))
	assert.Contains(t, reason, ID)
	assert.Contains(t, reason, prompt)
}

// A read-only brief that only sets the scope of its search declines no repair.
func TestAPromptThatSetsTheScopeOfASearchIsAllowed(t *testing.T) {
	for _, prompt := range []string{
		"Find where the guard decides to deny. Limit the search to the laziness and cmd packages, because the others are out of scope.",
		"Read how runTool builds its answer. The vendored grammars are out of scope for this search.",
	} {
		assert.Equal(t, Result{}, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt})), prompt)
	}
}

// quotedDenial spells out the ID and the advice, so it must change with them.
func TestTheDenialPatternNamesTheReason(t *testing.T) {
	assert.True(t, quotedDenial.MatchString(ID+`: a shape: "A sentence.". `+denialAdvice))
}

// A brief that quotes the guard's own denial reports the guard. It must not
// fire the guard a second time.
func TestAPromptThatQuotesADenialIsAllowed(t *testing.T) {
	denied := "The user authorized fixing wow-look-at-my/actions."
	reason := denyReason(t, run(t, toolCall(t, "Agent", map[string]any{"prompt": denied})))
	prompt := "The last brief was denied with this reason: " + reason + "\nFind the line in laziness/tool.go that writes it."
	assert.Equal(t, Result{}, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt})))
}

func TestAnAgentPromptNamingARealBlockerIsAllowed(t *testing.T) {
	prompt := "Fix the red build. go-toolchain master c021e219 never published because its host-build job failed checking out _gosmopolitan; fix that race in wow-look-at-my/actions first."
	assert.Equal(t, Result{}, run(t, toolCall(t, "Agent", map[string]any{"prompt": prompt, "description": "d"})))
}

func TestASendMessageAskingPermissionIsRepaired(t *testing.T) {
	in := map[string]any{"to": "a", "message": "Rebase the branch. I'm checking with the user first."}
	assert.Equal(t, "Rebase the branch.", repaired(t, run(t, toolCall(t, "SendMessage", in)))["message"])
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
		assert.Equal(t, Result{}, Run(strings.NewReader(body), cutPunts), body)
	}
}
