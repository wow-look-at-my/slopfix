// records.go is the raw view of the transcript the PreToolUse half needs.
// The Stop half works from turns, which keep only call signatures; deciding
// whether a status read can still learn anything needs the RESULT text too,
// because that is where a merge or a green build is reported.
package busypoll

import (
	"bytes"
	"encoding/json"
	"strings"
)

// toolCall is a tool_use block: the tool it names and the input it carries.
type toolCall struct {
	id    string
	name  string
	input json.RawMessage
}

// record is a JSONL line, reduced to what the decision reads, with raw kept whole for text matching.
type record struct {
	newPrompt bool
	wake      bool
	calls     []toolCall
	// failed names the tool_use ids whose result came back an error, so re-asking is not a repeat.
	failed []string
	// answered names every tool_use id a result has arrived for, error or not. A call with no result carries no state.
	answered []string
	raw      string
}

// wakeMarkers are the envelopes the harness delivers when something really happened, re-opening every subject.
var wakeMarkers = []string{
	"<wake reason=",
	"<task-notification>",
	"<webhook-payload>",
	"<event source=",
}

// interjectionMarker is how a message the user types MID-TURN reaches the transcript, folded into a tool_result.
const interjectionMarker = "the user sent a new message while you were working"

// parseRecords reads the tail of the transcript at path, keeping only what THIS session did.
// An unreadable transcript returns nil, which allows every call.
func parseRecords(path, sessionID string) []record {
	if path == "" {
		return nil
	}
	lines, err := readTail(path, transcriptTailBytes)
	if err != nil {
		return nil
	}

	var out []record
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var rec rawRecord
		if json.Unmarshal(line, &rec) != nil {
			continue
		}
		if rec.Sidechain {
			continue
		}
		if sessionID != "" && rec.SessionID != "" && rec.SessionID != sessionID {
			continue
		}
		r := record{raw: unescape(string(line))}
		lower := strings.ToLower(r.raw)
		for _, m := range wakeMarkers {
			if strings.Contains(lower, m) {
				r.wake = true
				break
			}
		}
		switch rec.Type {
		case "user":
			r.newPrompt = isNewPrompt(rec.Message.Content) || strings.Contains(lower, interjectionMarker)
			var blocks []rawBlock
			if json.Unmarshal(rec.Message.Content, &blocks) == nil {
				for _, b := range blocks {
					if b.Type != "tool_result" || b.ToolUseID == "" {
						continue
					}
					r.answered = append(r.answered, b.ToolUseID)
					if b.IsError {
						r.failed = append(r.failed, b.ToolUseID)
					}
				}
			}
		case "assistant":
			var blocks []rawBlock
			if json.Unmarshal(rec.Message.Content, &blocks) == nil {
				for _, b := range blocks {
					if b.Type == "tool_use" {
						r.calls = append(r.calls, toolCall{id: b.ID, name: b.Name, input: b.Input})
					}
				}
			}
		}
		out = append(out, r)
	}
	return out
}

// callText is the text a subject is read out of: the whole input for an MCP tool, and only the status statements for Bash.
func callText(c toolCall) string {
	if strings.EqualFold(c.name, "bash") {
		return c.name + " " + strings.Join(statusStatements(commandOf(c.input)), " ; ")
	}
	return c.name + " " + string(c.input)
}

// unescapeReplacer undoes a level of JSON string escaping. The \uXXXX
// entries cover the characters a Go encoder escapes by default and a
// JavaScript one leaves alone: a transcript written by either must read the
// same, or a wake envelope is invisible on one of them and the guard fails
// open without saying so.
var unescapeReplacer = strings.NewReplacer(
	`\"`, `"`,
	`\\`, `\`,
	`\n`, "\n",
	"\\u003c", "<", "\\u003C", "<",
	"\\u003e", ">", "\\u003E", ">",
	"\\u0026", "&",
)

// unescape flattens a record so a verdict inside a tool result is findable.
// A result's payload is a JSON string nested in the record's own JSON, so
// `{"outcome":"merged"}` reaches the transcript as `{\"outcome\":\"merged\"}`
// and matching the unescaped spelling finds nothing at all. This is text
// matching, not parsing: a single pass makes every nesting depth's quotes
// plain.
func unescape(line string) string {
	return unescapeReplacer.Replace(line)
}
