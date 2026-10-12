// Package rustfmt is the hook guard that makes rustfmt impossible to skip.
//
// PostToolUse on Write, Edit and MultiEdit formats the .rs file the tool wrote.
// PreToolUse on Bash formats every staged or modified .rs file before a git
// commit, and denies the commit. This happens when that changed a file or
// rustfmt failed.
package rustfmt

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Input is the part of the hook payload this guard reads.
type Input struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Cwd           string          `json:"cwd"`
}

// Result is what the guard answers. Exit 2 on PostToolUse puts the stderr in
// front of the model.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Run answers one hook payload. A payload it cannot read, or an event it does
// not serve, gets no answer.
func Run(r io.Reader) Result {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var in Input
	if json.Unmarshal(data, &in) != nil {
		return Result{}
	}
	switch in.HookEventName {
	case "PostToolUse":
		return postToolUse(in)
	case "PreToolUse", "":
		return preToolUse(in)
	}
	return Result{}
}

func postToolUse(in Input) Result {
	switch in.ToolName {
	case "Write", "Edit", "MultiEdit":
	default:
		return Result{}
	}
	var ti struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(in.ToolInput, &ti) != nil || !isRust(ti.FilePath) {
		return Result{}
	}
	changed, err := FormatFile(ti.FilePath)
	if err != nil {
		return Result{Stderr: fmt.Sprintf("rustfmt failed on %s. Fix the file so that rustfmt accepts it:\n%v\n", ti.FilePath, err), Code: 2}
	}
	if !changed {
		return Result{}
	}
	return Result{Stdout: encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":     "PostToolUse",
		"additionalContext": fmt.Sprintf("rustfmt reformatted %s after this edit. Read the file again before the next Edit to it.", ti.FilePath),
	}})}
}

func preToolUse(in Input) Result {
	if in.ToolName != "Bash" {
		return Result{}
	}
	var ti struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(in.ToolInput, &ti) != nil {
		return Result{}
	}
	var reasons []string
	for _, dir := range commitDirs(ti.Command, in.Cwd) {
		reasons = append(reasons, checkCommit(dir)...)
	}
	if len(reasons) == 0 {
		return Result{}
	}
	text := "Commit refused: rustfmt is not clean.\n  - " + strings.Join(reasons, "\n  - ") +
		"\nStage the rustfmt output with git add, then commit again."
	return Result{Stdout: encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       "deny",
		"permissionDecisionReason": text,
	}})}
}

func encode(v map[string]any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out) + "\n"
}
