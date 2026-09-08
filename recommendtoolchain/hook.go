// Package recommendtoolchain refuses a direct `go build` or `go test`. The
// repo's own wrapper, go-toolchain, builds, tests, vets and checks coverage in
// a single command, so a bare go invocation skips most of what a run has to do.
//
// Every failure path allows. A guard that blocks because it could not read its
// payload is worse than no guard.
package recommendtoolchain

import (
	"encoding/json"
	"io"
	"regexp"
)

// ID names the rule this package reports.
const ID = "toolchain/direct-go"

// Input is the subset of the PreToolUse payload this rule reads.
type Input struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	ToolInput     ToolInput `json:"tool_input"`
}

// ToolInput carries the Bash command the call is about to run.
type ToolInput struct {
	Command string `json:"command"`
}

// Result is what an invocation emits. A refusal is exit code 2 with the reason
// on stderr, which is how a PreToolUse hook hands the model its objection.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// allow lets the call run.
func allow() Result { return Result{} }

// goBuildPattern matches a direct build or test invocation of the go command.
var goBuildPattern = regexp.MustCompile(`\bgo\s+(build|test)\b`)

// message is what the model is told, naming the command to use instead.
const message = "BLOCKED: Direct `go build` and `go test` are not allowed. " +
	"Use `go-toolchain` instead -- it builds, tests, vets, and checks coverage in one command."

// Evaluate judges a raw hook payload.
func Evaluate(payload []byte) Result {
	var in Input
	if err := json.Unmarshal(payload, &in); err != nil {
		return allow()
	}
	if in.ToolName != "Bash" || in.ToolInput.Command == "" {
		return allow()
	}
	if !goBuildPattern.MatchString(in.ToolInput.Command) {
		return allow()
	}
	return Result{Code: 2, Stderr: message}
}

// Run reads a hook payload from r and judges it.
func Run(r io.Reader) Result {
	payload, _ := io.ReadAll(r)
	return Evaluate(payload)
}
