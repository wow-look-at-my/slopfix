// run.go is this package's entry point, and the reason the rule table is
// embedded rather than read from disk.
//
// The plugin used to ship rules.xml beside its binary and resolve the path from
// the executable path. slopfix is a single file, so there is no adjacent layout to resolve:
// the table travels inside the binary and a missing file cannot silently turn
// every rule off.
package autoallow

import (
	_ "embed"
	"encoding/json"
	"io"
)

//go:embed rules.xml
var rulesXML []byte

// Result is what the CLI prints and exits with.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Run reads a PermissionRequest or PreToolUse payload from r and returns the
// hook's response. Silence lets the call take the normal permission path.
func Run(r io.Reader) Result {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var hi HookInput
	if json.Unmarshal(data, &hi) != nil {
		return Result{}
	}
	if hi.HookEventName != eventPermissionRequest && hi.HookEventName != eventPreToolUse {
		return Result{}
	}
	// An allow on PreToolUse would settle the call before the user's own deny rules vote.
	denyOnly := hi.HookEventName == eventPreToolUse

	if hi.ToolName == "Read" || hi.ToolName == "Glob" || hi.ToolName == "Grep" {
		if denyOnly {
			return Result{}
		}
		return Result{Stdout: decisionPayload(hi.HookEventName, "allow", "")}
	}

	table, err := loadXMLRules(rulesXML)
	if err != nil {
		return Result{}
	}

	if server, tool := parseMCPTool(hi.ToolName); tool != "" {
		if !denyOnly && matchMCPServer(table.MCPServers, server, tool) {
			return Result{Stdout: decisionPayload(hi.HookEventName, "allow", "")}
		}
		return Result{}
	}
	if hi.ToolName != "Bash" {
		return Result{}
	}

	decision, message := evaluateCommandWith(hi.ToolInput.Command, table)
	if decision == "" || (denyOnly && decision != "deny") {
		return Result{}
	}
	return Result{Stdout: decisionPayload(hi.HookEventName, decision, message)}
}
