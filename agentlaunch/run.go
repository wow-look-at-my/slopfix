// Package agentlaunch repairs an Agent call that launches a read-only subagent
// in the foreground, or on the session's own model.
//
// A foreground Agent call holds the parent turn, and the queued messages of the
// user wait behind it. The built-in Explore agent declares `model: inherit`, so
// it runs on the session's model unless the call names one.
package agentlaunch

import (
	"encoding/json"
	"io"
	"os"

	"github.com/wow-look-at-my/go-containers/set"
)

// readOnly holds each built-in subagent type that cannot edit a file.
var readOnly = set.Of[string]("Explore", "Plan")

// searchModel is the model an Explore call gets when it names none.
const searchModel = "haiku"

// forceEnv names the variable that pins every subagent to one model.
const forceEnv = "CLAUDE_CODE_SUBAGENT_MODEL_FORCE"

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Run reads a PreToolUse payload. For an Agent call of a read-only type it
// answers updatedInput with `run_in_background` set to true where the call set
// it to false. An Explore call that names no model gets searchModel. Anything
// else answers nothing, which lets the call run as written.
func Run(r io.Reader) Result {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var in struct {
		HookEventName string         `json:"hook_event_name"`
		ToolName      string         `json:"tool_name"`
		ToolInput     map[string]any `json:"tool_input"`
	}
	if json.Unmarshal(data, &in) != nil || in.HookEventName != "PreToolUse" || in.ToolName != "Agent" || in.ToolInput == nil {
		return Result{}
	}
	input, changed := Repair(in.ToolInput, os.Getenv(forceEnv) != "")
	if !changed {
		return Result{}
	}
	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "updatedInput": input},
		"suppressOutput":     true,
	})
	if err != nil {
		return Result{}
	}
	return Result{Stdout: string(out)}
}

// Repair answers the input with the launch repaired, and whether it changed.
// The key `run_in_background` is only changed where the input holds it: Claude
// Code removes the key from the schema when background tasks are off. forced
// says that a variable pins the model, so the model stays as written.
func Repair(input map[string]any, forced bool) (map[string]any, bool) {
	kind, _ := input["subagent_type"].(string)
	if !readOnly.Contains(kind) {
		return input, false
	}
	changed := false
	if background, ok := input["run_in_background"].(bool); ok && !background {
		input["run_in_background"] = true
		changed = true
	}
	if _, named := input["model"]; kind == "Explore" && !named && !forced {
		input["model"] = searchModel
		changed = true
	}
	return input, changed
}
