package bashclean

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
)

// postInput is the part of the PostToolUse payload RunPost reads.
type postInput struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	Cwd           string `json:"cwd"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	ToolResponse struct {
		Stderr      string `json:"stderr"`
		Interrupted bool   `json:"interrupted"`
	} `json:"tool_response"`
}

// RunPost answers a PostToolUse payload. A Bash call that PlanRead maps gets
// its output replaced with the text the Read tool shows for the same lines,
// and a note names the Read calls. Every other payload gets silence.
func RunPost(r io.Reader) HookResult {
	data, err := io.ReadAll(r)
	if err != nil {
		return HookResult{}
	}
	var in postInput
	if json.Unmarshal(data, &in) != nil || in.HookEventName != "PostToolUse" || in.ToolName != "Bash" {
		return HookResult{}
	}
	if in.ToolResponse.Interrupted || in.ToolResponse.Stderr != "" {
		return HookResult{}
	}
	plan := PlanRead(in.ToolInput.Command, in.Cwd)
	if plan == nil {
		return HookResult{}
	}
	texts := make([]string, 0, len(plan.Reads))
	for _, read := range plan.Reads {
		text, ok := readText(read)
		if !ok {
			return HookResult{}
		}
		texts = append(texts, text)
	}
	stdout := texts[0]
	if len(texts) > 1 {
		parts := make([]string, len(texts))
		for i, text := range texts {
			parts[i] = "==> " + plan.Reads[i].FilePath + " <==\n" + text
		}
		stdout = strings.Join(parts, "\n\n")
	}
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"updatedToolOutput": map[string]any{"stdout": stdout, "stderr": "", "interrupted": false},
			"additionalContext": plan.Note,
		},
	}
	return HookResult{Stdout: encode(payload)}
}

// readText numbers the lines a Read call returns, the way the Read tool does:
// the line number, a tab, and the line with a trailing carriage return cut.
func readText(r ReadArgs) (string, bool) {
	data, err := os.ReadFile(r.FilePath)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(data), "\n")
	start := max(r.Offset, 1)
	end := len(lines)
	if r.Limit > 0 {
		end = min(end, start-1+r.Limit)
	}
	var b strings.Builder
	for n := start; n <= end; n++ {
		if n > start {
			b.WriteByte('\n')
		}
		b.WriteString(strconv.Itoa(n))
		b.WriteByte('\t')
		b.WriteString(strings.TrimSuffix(lines[n-1], "\r"))
	}
	return b.String(), true
}
