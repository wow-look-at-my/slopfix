// run.go is this package's hook entry point: the Stop payload in, a refusal
// out. Stop is the event, not MessageDisplay: this rule sends the model back
// to work, and a MessageDisplay annotation reaches only the reader.
package laziness

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Input is the part of the Stop payload this reads.
type Input struct {
	HookEventName  string `json:"hook_event_name"`
	StopHookActive bool   `json:"stop_hook_active"`
	// LastAssistantMessage is absent on some payloads, hence Transcript.
	LastAssistantMessage string `json:"last_assistant_message"`
	TranscriptPath       string `json:"transcript_path"`
}

// Result is what the CLI prints and exits with.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Run judges the turn's closing message and refuses a turn that reports a
// defect it did not fix. Every failure path allows the stop: a guard that
// wedges a session is worse than no guard.
func Run(r io.Reader) Result {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var in Input
	if json.Unmarshal(data, &in) != nil {
		return Result{}
	}
	if in.HookEventName != "" && in.HookEventName != "Stop" {
		return Result{}
	}
	// Fires at most per turn: a message that cannot be rewritten would
	// otherwise trip this forever.
	if in.StopHookActive {
		return Result{}
	}

	message := in.LastAssistantMessage
	if message == "" {
		message = lastAssistantInTranscript(in.TranscriptPath)
	}
	if message == "" {
		return Result{}
	}
	if len(Check(message)) == 0 {
		return Result{}
	}
	return Result{Stderr: "continue", Code: 2}
}

// transcriptEntry is a JSONL line. The content is a string on some entries and
// a list of typed parts on others, so it is decoded either way.
type transcriptEntry struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Message json.RawMessage `json:"message"`
	Content json.RawMessage `json:"content"`
}

// lastAssistantInTranscript reads the newest assistant text. The file is
// JSONL, newest last. An unreadable path answers with nothing, which leaves
// the turn unjudged rather than refused.
func lastAssistantInTranscript(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	found := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry transcriptEntry
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		if entry.Type != "assistant" && entry.Role != "assistant" {
			continue
		}
		raw := entry.Content
		if len(entry.Message) > 0 {
			var inner struct {
				Content json.RawMessage `json:"content"`
			}
			if json.Unmarshal(entry.Message, &inner) == nil && len(inner.Content) > 0 {
				raw = inner.Content
			}
		}
		if text := textOf(raw); text != "" {
			found = text
		}
	}
	return found
}

// textOf reads the content field, which carries either a string or the typed
// parts of a message. Anything that is not text contributes nothing.
func textOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
