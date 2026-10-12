package noworkloss

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
)

// foundBeforeTool runs Found work on PreToolUse. It records what the call
// touches. Before a command that sets work aside, it commits and pushes that
// work. A git add that would record an undeclared gitlink comes back
// rewritten in updated.
func foundBeforeTool(in hookInput) (notices []string, updated *string) {
	if in.ToolName != "Bash" {
		return nil, nil
	}
	var ti toolInput
	if json.Unmarshal(in.ToolInput, &ti) != nil || strings.TrimSpace(ti.Command) == "" {
		return nil, nil
	}
	touched, trig := foundCommand(ti.Command, in.Cwd)
	recordRepos(in.SessionID, touched)
	for root, pattern := range trig.exclude {
		if err := excludePath(root, pattern); err != nil {
			notices = append(notices, "no-work-loss: cannot add "+pattern+" to info/exclude in "+root+": "+err.Error())
		}
	}
	var roots []string
	for root, nested := range trig.embedded {
		for _, p := range nested {
			if err := excludePath(root, "/"+p+"/"); err != nil {
				notices = append(notices, "no-work-loss: cannot add /"+p+"/ to info/exclude in "+root+": "+err.Error())
			}
			roots = append(roots, filepath.Join(root, p))
		}
	}
	for _, r := range trig.roots {
		if !slices.Contains(roots, r) {
			roots = append(roots, r)
		}
	}
	for _, root := range roots {
		for _, res := range commitFound(root, in.SessionID, 0) {
			notices = append(notices, res.report())
		}
	}
	if len(trig.dropOperands) > 0 {
		cmd := dropWords(ti.Command, trig.dropOperands)
		updated = &cmd
	}
	return notices, updated
}

// foundAfterTool runs Found work on PostToolUse. A tool that names a path
// records the repository that holds it. A Bash command whose output showed
// found work commits and pushes that work.
func foundAfterTool(in hookInput) Result {
	var ti toolInput
	_ = json.Unmarshal(in.ToolInput, &ti)
	switch in.ToolName {
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit", "Grep", "Glob":
		var dirs []string
		for _, p := range []string{ti.FilePath, ti.NotebookPath, ti.Path} {
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(in.Cwd, p)
			}
			dirs = append(dirs, p)
		}
		if (in.ToolName == "Grep" || in.ToolName == "Glob") && ti.Path == "" {
			dirs = append(dirs, in.Cwd)
		}
		recordRepos(in.SessionID, dirs)
		return Result{}
	case "Bash":
	default:
		return Result{}
	}
	touched, _ := foundCommand(ti.Command, in.Cwd)
	roots := recordRepos(in.SessionID, touched)
	if !showsFoundWork(toolOutput(in.ToolResponse)) {
		return Result{}
	}
	var notices []string
	for _, root := range roots {
		for _, res := range commitFound(root, in.SessionID, 0) {
			notices = append(notices, res.report())
		}
	}
	if len(notices) == 0 {
		return Result{}
	}
	return Result{Stdout: contextPayload("PostToolUse", notices)}
}

// foundAtStop commits and pushes the found work of every repository the
// session touched. Work that cannot reach a remote refuses the stop.
func foundAtStop(in hookInput) Result {
	if in.SessionID == "" {
		return Result{}
	}
	var failed, done []string
	for _, root := range readFoundState(in.SessionID).Repos {
		for _, res := range commitFound(root, in.SessionID, 0) {
			if res.failure != "" {
				failed = append(failed, res.report())
				continue
			}
			done = append(done, res.report())
		}
	}
	if len(failed) > 0 {
		return Result{Stdout: encodeJSON(map[string]any{
			"decision": "block",
			"reason":   strings.Join(append(failed, done...), "\n"),
		})}
	}
	if len(done) > 0 {
		return Result{Stdout: encodeJSON(map[string]any{"systemMessage": strings.Join(done, "\n")})}
	}
	return Result{}
}

// toolOutput reads what a Bash call printed, from either shape of tool_response.
func toolOutput(raw json.RawMessage) string {
	var shaped struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
		Output string `json:"output"`
	}
	if json.Unmarshal(raw, &shaped) == nil {
		return shaped.Stdout + "\n" + shaped.Stderr + "\n" + shaped.Output
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}

func encodeJSON(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}

// contextPayload tells the model what the guard did, on an event that takes additionalContext.
func contextPayload(event string, notices []string) string {
	return encodeJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": strings.Join(notices, "\n"),
		},
	})
}

// withCommand answers a PreToolUse payload with its command replaced.
func withCommand(raw []byte, command string) []byte {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil {
		return raw
	}
	input, _ := envelope["tool_input"].(map[string]any)
	if input == nil {
		return raw
	}
	input["command"] = command
	out, err := json.Marshal(envelope)
	if err != nil {
		return raw
	}
	return out
}

// rewritePayload carries the rewritten tool input, and a notice when the guard did something.
func rewritePayload(raw []byte, notices []string) string {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil {
		return ""
	}
	spec := map[string]any{"hookEventName": "PreToolUse", "updatedInput": envelope["tool_input"]}
	out := map[string]any{"hookSpecificOutput": spec}
	if len(notices) > 0 {
		out["systemMessage"] = strings.Join(notices, "\n")
	}
	return encodeJSON(out)
}
