// Package autoallow decides Bash and MCP tool permissions from two hook
// events. It auto-approves read-only work, and it refuses programs this
// environment does not allow to run.
//
// The split between the events is load-bearing. PermissionRequest fires only
// after the permission engine has already landed on "ask", so a deny riding it
// alone stays silent under an auto default mode. PreToolUse fires on every tool
// call, so the deny half rides that. The allow half stays OUT of PreToolUse: an
// allow there settles the call before the user's own deny rules vote.
//
// Every failure path stays silent, which leaves the normal permission flow
// untouched.
package autoallow

import (
	_ "embed"
	"encoding/json"
	"io"
	"sync"
)

// ID names the rule this package reports.
const ID = "permissions/auto-allow"

// rulesXML is the shipped rule set. It travels with the package so a rule and
// its embedded proof cannot drift apart.
//
//go:embed rules.xml
var rulesXML []byte

// Shipped returns the parsed shipped rule set. A malformed file yields an empty
// set, which decides nothing.
var Shipped = sync.OnceValue(func() Rules {
	r, err := loadXMLRules(rulesXML)
	if err != nil {
		return Rules{}
	}
	return r
})

// RulesXML hands back the embedded file itself, for the tests that read it.
func RulesXML() []byte { return rulesXML }

// Input is the subset of the payloads this rule reads. Both events arrive on
// the same command, because the question is the same asked twice.
type Input struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	ToolInput     ToolInput `json:"tool_input"`
}

// ToolInput carries the Bash command a call is about to run.
type ToolInput struct {
	Command string `json:"command"`
}

// Result is what an invocation emits. A verdict is a JSON payload on stdout.
// Silence is the empty result.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// silent leaves the call to the normal permission flow.
func silent() Result { return Result{} }

// The events this package answers. They are NOT interchangeable.
const (
	eventPermissionRequest = "PermissionRequest"
	eventPreToolUse        = "PreToolUse"
)

// PermissionResponse is the PermissionRequest reply shape.
type PermissionResponse struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
		Decision      struct {
			Behavior string `json:"behavior"`
			Message  string `json:"message,omitempty"`
		} `json:"decision"`
	} `json:"hookSpecificOutput"`
}

// PreToolUseResponse is the PreToolUse reply shape: a flat permissionDecision,
// not the nested object. The CLI rejects a hookEventName that is not the event
// it dispatched, so the shape follows the event rather than the verdict.
type PreToolUseResponse struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// Run reads a hook payload from r and dispatches on the event it carries.
func Run(r io.Reader) Result {
	data, _ := io.ReadAll(r)
	var in Input
	if err := json.Unmarshal(data, &in); err != nil {
		return silent()
	}
	return Judge(in, Shipped())
}

// Judge answers a payload against a rule set. Tests hand their own set here, so
// nothing has to swap a package-level value and put it back.
func Judge(in Input, rules Rules) Result {
	if in.HookEventName != eventPermissionRequest && in.HookEventName != eventPreToolUse {
		return silent()
	}

	// An allow on PreToolUse would settle the call before the user's own deny
	// rules vote, so that event carries denials alone.
	denyOnly := in.HookEventName == eventPreToolUse

	switch in.ToolName {
	case "Read", "Glob", "Grep":
		if denyOnly {
			return silent()
		}
		return decision(in.HookEventName, "allow", "")
	}

	if server, tool := parseMCPTool(in.ToolName); tool != "" {
		if !denyOnly && matchMCPServer(rules.MCPServers, server, tool) {
			return decision(in.HookEventName, "allow", "")
		}
		return silent()
	}

	if in.ToolName != "Bash" {
		return silent()
	}

	behavior, message := evaluateCommandWith(in.ToolInput.Command, rules)
	if behavior == "" || (denyOnly && behavior != "deny") {
		return silent()
	}
	return decision(in.HookEventName, behavior, message)
}

// decision renders the reply shape the dispatched event demands.
func decision(event, behavior, message string) Result {
	var payload any
	if event == eventPreToolUse {
		resp := PreToolUseResponse{}
		resp.HookSpecificOutput.HookEventName = eventPreToolUse
		resp.HookSpecificOutput.PermissionDecision = behavior
		resp.HookSpecificOutput.PermissionDecisionReason = message
		payload = resp
	} else {
		resp := PermissionResponse{}
		resp.HookSpecificOutput.HookEventName = eventPermissionRequest
		resp.HookSpecificOutput.Decision.Behavior = behavior
		resp.HookSpecificOutput.Decision.Message = message
		payload = resp
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return silent()
	}
	return Result{Stdout: string(out) + "\n"}
}
