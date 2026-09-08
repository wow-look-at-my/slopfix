// run.go is this package's entry point: a PreToolUse payload in, the hook's
// response out.
//
// A rewrite carries updatedInput and nothing else. It sets no
// permissionDecision, so the normal permission flow still judges the rewritten
// command, and it says nothing to the model: a visible hook message lets the
// model blame the hook for its own command mistakes.
package bashclean

import (
	"encoding/json"
	"io"
)

// Input is the part of the PreToolUse payload this reads.
type Input struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// HookResult is what the CLI prints and exits with.
type HookResult struct {
	Stdout string
	Stderr string
	Code   int
}

// Run reads a payload from r and returns the hook's response. Silence lets the
// command through untouched, which is every failure path as well.
func Run(r io.Reader) HookResult {
	data, err := io.ReadAll(r)
	if err != nil {
		return HookResult{}
	}
	var in Input
	if json.Unmarshal(data, &in) != nil {
		return HookResult{}
	}
	if in.HookEventName != "" && in.HookEventName != "PreToolUse" {
		return HookResult{}
	}
	if in.ToolName != "Bash" {
		return HookResult{}
	}
	var raw map[string]any
	if json.Unmarshal(in.ToolInput, &raw) != nil {
		return HookResult{}
	}
	command, _ := raw["command"].(string)
	if command == "" {
		return HookResult{}
	}

	res := Transform(command)
	switch {
	case res.Denied:
		return HookResult{Stdout: denyPayload(res.Reason)}
	case res.Changed:
		raw["command"] = res.Command
		return HookResult{Stdout: rewritePayload(raw)}
	}
	return HookResult{}
}

func denyPayload(reason string) string {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}
	return encode(payload)
}

func rewritePayload(raw map[string]any) string {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse",
			"updatedInput":  raw,
		},
		"suppressOutput": true,
	}
	return encode(payload)
}

func encode(payload map[string]any) string {
	out, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(out)
}
