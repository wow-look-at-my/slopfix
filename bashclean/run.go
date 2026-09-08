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
	"fmt"
	"io"
	"os"
	"strings"
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
		logLine("DENY\toriginal=%q\treason=%q\n", command, res.Reason)
		return HookResult{Stdout: denyPayload(denyReason(res.Reason))}
	case res.Changed:
		logLine("REWRITE\toriginal=%q\tcleaned=%q\trules=%q\n", command, res.Command, strings.Join(res.Rules, ","))
		raw["command"] = res.Command
		return HookResult{Stdout: rewritePayload(raw)}
	}
	return HookResult{}
}

// The hook is silent toward user and model, so CLEANUP_BASH_CMDS_LOG is the
// only debug channel. A failure to write it never breaks the hook.
func logLine(format string, args ...any) {
	path := os.Getenv("CLEANUP_BASH_CMDS_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, format, args...)
}

// denyReason turns a rule name into the sentence the model reads. Each names
// the alternative that works, or the deny teaches nothing and the model
// retries the same command.
func denyReason(rule string) string {
	switch rule {
	case "perl":
		return "perl is banned in this environment."
	case "file_read":
		return "Reading files with cat/head/tail, or with sed -n selecting lines, is banned in this environment. Use the Read tool instead: its offset and limit parameters read part of a file, which is what head/tail/sed -n were reached for. Only /proc, /sys, and /dev pseudo-files are exempt."
	case "shred":
		return "shred/srm destroy data unrecoverably by design and are banned in this environment. There is no safe equivalent; if the file must go, use recycler trash <path> and it can be restored."
	case "git_rm":
		return "git rm deletes the working-tree file and is banned in this environment. Use recycler trash <path> && git add -A instead. (git rm --cached only unstages and is allowed.)"
	case "truncate_zero":
		return "truncate -s 0 empties a file in place, destroying its contents unrecoverably. Use recycler trash <path> instead, which moves it to the recycle bin."
	case "rm_flag":
		return "This rm carries a flag that cannot be translated to recycler trash. rm is rewritten to recycler trash (which understands only paths), so only -r/-R/--recursive, -f/--force, -v/--verbose, -i/-I/--interactive, and -- are accepted. Rerun with just the paths."
	}
	return "Heredocs are banned in this environment. Write file content with the Write/Edit tools; for command stdin use printf '%s' ... | cmd or a temp file."
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
