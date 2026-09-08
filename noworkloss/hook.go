// no-work-loss: a PreToolUse hook that refuses the ways a session loses
// authorship of the working tree.
//
//   - Destruction: a command that would destroy content existing only in the
//     working tree. A modified-but-uncommitted or untracked file is in no git
//     object, so losing it is unrecoverable; committed work is reachable from
//     the reflog and is deliberately NOT protected.
//   - Provenance: a change to file content that does not go through Write, Edit
//     or NotebookEdit. Bash runs things -- git, builds, tests, validation,
//     search -- and does not author files.
//
// Both questions are asked of the same parsed command, which is why they live in
// a single plugin: the shell walk, the wrapper stripping and the path
// resolution are the same machinery, and separate copies of it would drift.
// see docs/decision-model.md and docs/write-routes.md
package noworkloss

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// IDDestruction names the refusal of a command destroying working-tree content.
const IDDestruction = "noworkloss/destruction"

// IDProvenance names the refusal of a change routed around the edit tools.
const IDProvenance = "noworkloss/provenance"

// IDWriteTool names the refusal of a Write over a path that already holds content.
const IDWriteTool = "noworkloss/write-tool"

type hookInput struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	Cwd           string          `json:"cwd"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// Result is what an invocation emits. A refusal rides stdout as a deny
// payload, which is how a PreToolUse hook stops a call before it runs.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// The CLI rejects a payload whose hookEventName is not the event it
// dispatched, so the shape follows the event rather than the verdict.
type preToolUseResponse struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// preToolUseNotice carries no permissionDecision: a preservation leaves the
// permission flow untouched and only says where the content went.
type preToolUseNotice struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
	} `json:"hookSpecificOutput"`
	SystemMessage string `json:"systemMessage"`
}

// Run reads a PreToolUse payload from r and decides the call it describes.
func Run(r io.Reader) Result {
	raw, err := io.ReadAll(r)
	if err != nil {
		// Reading the payload failed, so nothing is known about the call and
		// there is no reason to attach to a decision.
		return Result{}
	}
	reason, notices := decide(raw)
	if reason != "" {
		return Result{Stdout: denyPayload(reason)}
	}
	// A preservation moved content into a ref, which must never happen silently.
	if len(notices) > 0 {
		return Result{Stdout: noticePayload(notices)}
	}
	return Result{}
}

// evaluateLoss runs the destruction analysis under a recover, failing OPEN on a panic.
func evaluateLoss(command, cwd string) (reason string, notices []string) {
	// A cheap byte scan leads: the overwhelming majority of Bash calls name no verb
	// that can delete anything, and those must not pay for a parse or a
	// subprocess.
	if command == "" || !mayDestroy(command) {
		return "", nil
	}
	defer func() {
		if r := recover(); r != nil {
			reason, notices = "", nil
			if verb, ok := destructiveKeyword(command); ok {
				reason = internalErrorReason(verb)
			}
		}
	}()
	return analyze(command, cwd)
}

func denyPayload(reason string) string {
	var resp preToolUseResponse
	resp.HookSpecificOutput.HookEventName = "PreToolUse"
	resp.HookSpecificOutput.PermissionDecision = "deny"
	resp.HookSpecificOutput.PermissionDecisionReason = reason
	out, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(out)
}

// emitDeny writes a denial to stdout, which is what the raw-byte tests drive.
func emitDeny(reason string) {
	if out := denyPayload(reason); out != "" {
		os.Stdout.WriteString(out)
	}
}

func noticePayload(notices []string) string {
	var resp preToolUseNotice
	resp.HookSpecificOutput.HookEventName = "PreToolUse"
	resp.SystemMessage = strings.Join(notices, "\n")
	out, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(out)
}
