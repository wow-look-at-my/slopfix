// Package askproperly is a MessageDisplay hook. It marks a closing message
// that hands the user a decision in prose, by appending a line to what the
// reader sees. It sends NOTHING back to the model.
//
// A question typed into a closing message is a decision handed back: the user
// works out which options were meant and types an answer. AskUserQuestion
// renders the choices instead, so the model has to have thought them through.
//
// A Stop hook cannot do this job. It runs AFTER the message streams, so
// refusing unsends nothing: the retype puts the decision back into prose and
// fires the guard again, with no bound.
//
// displayContent is display-only: it replaces the delta on screen without
// changing the stored message. Every failure path prints nothing.
package askproperly

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// HookInput is the subset of the MessageDisplay payload this plugin reads.
// `delta` is whole lines except on the final flush, and `final` is the
// end-of-message signal even when its delta is empty.
type HookInput struct {
	HookEventName  string `json:"hook_event_name"`
	TranscriptPath string `json:"transcript_path"`
	MessageID      string `json:"message_id"`
	Index          int    `json:"index"`
	Final          bool   `json:"final"`
	Delta          string `json:"delta"`
}

// Output is what the CLI reads back. displayContent replaces this flush on
// screen, and is the only field the event's output schema carries.
type Output struct {
	HookSpecificOutput struct {
		HookEventName  string `json:"hookEventName"`
		DisplayContent string `json:"displayContent"`
	} `json:"hookSpecificOutput"`
}

// Result is what an invocation emits. This rule refuses nothing, so the code is
// always a success and stderr is always empty.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Run reads a MessageDisplay payload from r. An empty Stdout means print
// nothing, which shows the original text.
func Run(r io.Reader) Result {
	return Result{Stdout: run(r)}
}

// run decides what this flush renders as. An empty return means print nothing.
//
// The message is judged whole, on its last flush: a question can span a line
// wrap, so judging a flush alone would miss it and mark the message again.
func run(r io.Reader) string {
	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return ""
	}
	var in HookInput
	if err := json.Unmarshal(data, &in); err != nil {
		return ""
	}
	if in.HookEventName != "MessageDisplay" || disabled() {
		return ""
	}

	prior := ""
	if in.MessageID != "" {
		prior = priorText(in.MessageID)
	}
	if !in.Final {
		if in.MessageID != "" {
			rememberText(in.MessageID, prior+in.Delta)
		}
		return ""
	}
	if in.MessageID != "" {
		forgetText(in.MessageID)
		sweep()
	}

	// A turn that already used the tool asked properly, so prose beside a
	// rendered card is commentary rather than an offloaded decision.
	if ReadTurn(in.TranscriptPath).UsedAskTool {
		return ""
	}

	note := Annotate(prior + in.Delta)
	if note == "" {
		return ""
	}

	var out Output
	out.HookSpecificOutput.HookEventName = "MessageDisplay"
	out.HookSpecificOutput.DisplayContent = in.Delta + note
	encoded, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func disabled() bool {
	switch os.Getenv("CC_ASK_PROPERLY") {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

var stateDir = filepath.Join(os.TempDir(), "cc-ask-properly")

// unsafeName strips what is not a filename character: an id is never trusted into a path.
var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func stateFile(messageID string) string {
	return filepath.Join(stateDir, unsafeName.ReplaceAllString(messageID, "")+".txt")
}

// priorText is this message's text before the current flush. Losing it costs
// the earlier flushes' questions, never a wrong annotation.
func priorText(messageID string) string {
	data, err := os.ReadFile(stateFile(messageID))
	if err != nil {
		return ""
	}
	return string(data)
}

func rememberText(messageID, text string) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(stateFile(messageID), []byte(text), 0o600)
}

func forgetText(messageID string) { _ = os.Remove(stateFile(messageID)) }

// sweep collects what a session that died mid-message left behind, so the
// directory cannot grow without bound.
func sweep() {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > time.Hour {
			_ = os.Remove(filepath.Join(stateDir, entry.Name()))
		}
	}
}
