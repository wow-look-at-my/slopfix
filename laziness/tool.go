// toolInput is the part of a PreToolUse payload this reads.

package laziness

import (
	"encoding/json"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// toolInput is the part of a PreToolUse payload this reads.
type toolInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// proseOf calls visit on each string of a tool call that a reader sees. This
// covers the questions and options of AskUserQuestion, the prompt of Agent and
// every string of the message of SendMessage. visit answers the new value of
// the string. Any other tool visits nothing.
func proseOf(name string, input map[string]any, visit func(string) string) {
	field := func(m map[string]any, key string) {
		if s, ok := m[key].(string); ok {
			m[key] = visit(s)
		}
	}
	switch name {
	case "AskUserQuestion":
		questions, _ := input["questions"].([]any)
		for _, q := range questions {
			question, ok := q.(map[string]any)
			if !ok {
				continue
			}
			field(question, "question")
			options, _ := question["options"].([]any)
			for _, o := range options {
				if option, ok := o.(map[string]any); ok {
					field(option, "label")
					field(option, "description")
				}
			}
		}
	case "Agent":
		field(input, "prompt")
	case "SendMessage":
		if message, ok := input["message"]; ok {
			input["message"] = eachString(message, visit)
		}
	}
}

// eachString answers v with visit applied to every string it holds, at any
// depth. A message can be plain text or a structured object.
func eachString(v any, visit func(string) string) any {
	switch t := v.(type) {
	case string:
		return visit(t)
	case []any:
		for i, e := range t {
			t[i] = eachString(e, visit)
		}
	case map[string]any:
		for k, e := range t {
			t[k] = eachString(e, visit)
		}
	}
	return v
}

// denialAdvice closes the reason of a denial.
const denialAdvice = "A fix needs no one's permission. Do the work, or name the real blocker plainly."

// readByUser names each tool whose prose the user reads.
var readByUser = set.Of("AskUserQuestion")

// runTool repairs an agent prompt or a message that asks permission to do a
// fix, or frames a fix as something the user grants. repair cuts each sentence
// that carries a tell, and the call runs with the repaired input. The guard
// denies a call the user reads, and a call the cut leaves with an empty string.
func runTool(data []byte, repair func(string) string) Result {
	var in toolInput
	if json.Unmarshal(data, &in) != nil {
		return Result{}
	}
	var input map[string]any
	if json.Unmarshal(in.ToolInput, &input) != nil {
		return Result{}
	}
	var first *Hit
	deny := false
	proseOf(in.ToolName, input, func(s string) string {
		hits := Check(s)
		if len(hits) == 0 {
			return s
		}
		if first == nil {
			first = &hits[0]
		}
		if readByUser.Contains(in.ToolName) {
			deny = true
			return s
		}
		fixed := repair(s)
		if strings.TrimSpace(fixed) == "" || len(Check(fixed)) > 0 {
			deny = true
		}
		return fixed
	})
	if first == nil {
		return Result{}
	}
	output := map[string]any{"hookEventName": "PreToolUse"}
	if deny {
		output["permissionDecision"] = "deny"
		output["permissionDecisionReason"] = ID + ": " + first.Tell + ": \"" + first.Sentence + "\". " + denialAdvice
	} else {
		output["updatedInput"] = input
	}
	out, err := json.Marshal(map[string]any{"hookSpecificOutput": output, "suppressOutput": !deny})
	if err != nil {
		return Result{}
	}
	return Result{Stdout: string(out)}
}
