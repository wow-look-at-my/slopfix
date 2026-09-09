// Package busypoll refuses a turn that is the latest in a run of several
// turns making the exact same tool call, closely spaced in time, with nothing
// else different in between. It also refuses the status read itself, before
// it runs, when nothing it could return has changed.
//
// That shape is what a manual polling loop looks like: re-running the
// same status check every turn instead of waiting for a real event or arming
// an actual scheduled wakeup. It burns the user's tokens for no new
// information, because the answer cannot have changed between calls seconds
// apart.
//
// A properly spaced watch loop -- the same check re-run after a real gap,
// because a scheduled trigger fired or an event arrived -- is not that
// pattern and is not refused. See detect.go for the spacing rule that tells
// them apart.
//
// A refusal that says "wait for the event" needs events to exist, so both
// rules that say it run in a remote session only. See environment.go.
//
// Every failure path allows. A guard that blocks because it could not read a
// file is worse than no guard.
package busypoll

import (
	"encoding/json"
	"io"
	"strings"
	"text/template"
)

// Input is the subset of the payloads this rule reads; both events arrive on the same command.
type Input struct {
	HookEventName  string          `json:"hook_event_name"`
	TranscriptPath string          `json:"transcript_path"`
	SessionID      string          `json:"session_id"`
	StopHookActive bool            `json:"stop_hook_active"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
}

// Result is what an invocation emits: a stop refuses on stderr, a tool call refuses with a deny payload.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// allow lets the turn end, or the call run.
func allow() Result { return Result{} }

// Run reads a hook payload from r and dispatches on the event it carries.
func Run(r io.Reader) Result {
	data, _ := io.ReadAll(r)
	var in Input
	if err := json.Unmarshal(data, &in); err != nil {
		return allow()
	}
	switch in.HookEventName {
	case "", "Stop":
		return runStop(in)
	case "PreToolUse":
		return runPreTool(in)
	}
	return allow()
}

// runStop refuses to END a turn that is the latest in a run of identical,
// closely-spaced turns.
func runStop(in Input) Result {
	if !remoteSession() {
		return allow()
	}
	n, calls := streak(parseTurns(in.TranscriptPath))
	if n < threshold() {
		return allow()
	}
	return Result{Code: 2, Stderr: reason(n, calls, in.StopHookActive)}
}

// runPreTool refuses a status read that cannot learn anything, before it runs.
func runPreTool(in Input) Result {
	if in.ToolName == "" {
		return allow()
	}
	v := judgeCall(toolCall{name: in.ToolName, input: in.ToolInput}, parseRecords(in.TranscriptPath, in.SessionID))
	if !v.deny {
		return allow()
	}
	return Result{Stdout: denyPayload(v.reason)}
}

// reason is what the model is told. It names the repeated call, states the
// count, and gives the ways out, because a refusal that does not say what to
// do instead just gets repeated with a different excuse.
func reason(n int, calls []call, repeat bool) string {
	shown := make([]string, 0, len(calls))
	for _, c := range calls {
		shown = append(shown, c.disp)
	}
	var b strings.Builder
	if err := reasonTemplate.Execute(&b, struct {
		N      int
		Calls  []string
		Repeat bool
	}{n, shown, repeat}); err != nil {
		panic("busypoll: the refusal template does not render: " + err.Error())
	}
	return b.String()
}

// reasonTemplate is the refusal as a document. It reads as the text it produces,
// which a run of writes does not.
var reasonTemplate = template.Must(template.New("reason").Parse(
	`Stop. The last {{.N}} turns in a row made the exact same call, with nothing else
different in between and no real wait between them -- that is a busy-poll loop,
and it burns the user's tokens for zero new signal, because the answer cannot
have changed in the seconds since the last check:

{{range .Calls}}  {{.}}
{{end}}
Do not run any of the calls above again on a hunch. Either:

  - Reply with NO tool call at all and wait for a real signal -- a queued
    notification, a scheduled trigger firing, an actual event arriving -- or
  - Arm a real wakeup (ScheduleWakeup / send_later / a Monitor watch) with a
    genuine delay, then stop. Never re-check by hand in the meantime.

Rewrite this turn so it makes none of the calls listed above, then stop.{{if .Repeat}}

This is not the first refusal. Making the same call again will refuse again --
the fix is to stop calling it, not to call it one more time.{{end}}`))
