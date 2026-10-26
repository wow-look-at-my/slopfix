// toolInput is the part of a PreToolUse payload this reads.

package laziness

import (
	"encoding/json"
	"strings"
)

// toolInput is the part of a PreToolUse payload this reads.
type toolInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// toolText answers the prose a tool call puts in front of a reader. This covers
// the questions and options of AskUserQuestion, the prompt of Agent, the
// message of SendMessage. Any other tool answers nothing.
func toolText(name string, raw json.RawMessage) string {
	switch name {
	case "AskUserQuestion":
		var in struct {
			Questions []struct {
				Question string `json:"question"`
				Header   string `json:"header"`
				Options  []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
			} `json:"questions"`
		}
		if json.Unmarshal(raw, &in) != nil {
			return ""
		}
		var parts []string
		for _, q := range in.Questions {
			parts = append(parts, q.Question)
			for _, o := range q.Options {
				parts = append(parts, o.Label+". "+o.Description)
			}
		}
		return strings.Join(parts, "\n")
	case "Agent":
		var in struct {
			Prompt string `json:"prompt"`
		}
		if json.Unmarshal(raw, &in) != nil {
			return ""
		}
		return in.Prompt
	case "SendMessage":
		var in struct {
			Message json.RawMessage `json:"message"`
		}
		if json.Unmarshal(raw, &in) != nil {
			return ""
		}
		return strings.Join(stringsIn(in.Message), "\n")
	}
	return ""
}

// stringsIn answers every string a JSON value holds, at any depth. A message
// can be plain text or a structured object.
func stringsIn(raw json.RawMessage) []string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			out = append(out, t)
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	return out
}

// runTool refuses a question, an agent prompt or a message that asks
// permission to do a fix. It frames a fix as something the user grants.
func runTool(data []byte) Result {
	var in toolInput
	if json.Unmarshal(data, &in) != nil {
		return Result{}
	}
	text := toolText(in.ToolName, in.ToolInput)
	if text == "" {
		return Result{}
	}
	hits := Check(text)
	if len(hits) == 0 {
		return Result{}
	}
	reason := ID + ": " + hits[0].Tell + ": \"" + hits[0].Sentence + "\". A fix needs no one's permission. Do the work, or name the real blocker plainly."
	out, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       "deny",
		"permissionDecisionReason": reason,
	}})
	if err != nil {
		return Result{}
	}
	return Result{Stdout: string(out)}
}
