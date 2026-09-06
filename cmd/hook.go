package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
	"github.com/wow-look-at-my/slopfmt/tombstones"
)

var (
	// hookOnly restricts the run to the rules a caller names.
	hookOnly []string
	// hookMaxLines caps a comment block.
	hookMaxLines int
)

func init() {
	hook := &cobra.Command{
		Use:   "hook",
		Short: "Repair a Claude Code write, reading its PreToolUse payload on stdin",
		Long: "hook reads a PreToolUse payload on stdin and writes the hook's own\n" +
			"response on stdout. It repairs the text a write adds and lets the write\n" +
			"through, and refuses only what no rewrite can repair.\n\n" +
			"Every plugin that guards a write is then its manifest and nothing else.\n" +
			"Each carried its own copy of this: the same payload parse, the same three\n" +
			"write shapes, the same splice back. A write shape added to one and not\n" +
			"the other is a guard that silently stops seeing half the writes.\n\n" +
			"It prints nothing, and exits 0, for anything it does not judge. That\n" +
			"covers an unreadable payload, another event, a tool that writes no file,\n" +
			"and a path whose rules leave the text alone.",
		Args: cobra.NoArgs,
		RunE: runHook,
	}
	hook.Flags().StringSliceVar(&hookOnly, "only", nil,
		"run only these, as a comma-separated list. An entry is a category ("+
			strings.Join(ruleNames(), ", ")+") or a single rule ID, which is the name the report prints")
	hook.Flags().IntVar(&hookMaxLines, "max-comment-lines", tombstones.DefaultMaxCommentLines, "cap a comment block, 0 to turn the cap off")
	rootCmd.AddCommand(hook)
}

// hookInput is the part of the PreToolUse payload this reads.
type hookInput struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// writeInput carries the shapes of Write, Edit and MultiEdit together. Each
// tool fills the fields it has and leaves the rest empty.
type writeInput struct {
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
	NewString string `json:"new_string"`
	Edits     []struct {
		NewString string `json:"new_string"`
	} `json:"edits"`
}

// hookResponse covers a refusal, which sets the permission fields, and a
// repair, which sets the input and the notice. Allowing an untouched write
// prints nothing at all.
type hookResponse struct {
	HookSpecificOutput struct {
		HookEventName            string         `json:"hookEventName"`
		PermissionDecision       string         `json:"permissionDecision,omitempty"`
		PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
		UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
		AdditionalContext        string         `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

// unit is a string a write replaces. apply puts the repaired text back into the
// payload as it arrived, so every other field survives.
type unit struct {
	text  string
	apply func(string)
}

// writeUnits reads the units the named tool's own shape carries.
func writeUnits(tool string, in writeInput, raw map[string]any) []unit {
	switch tool {
	case "Write":
		return []unit{{text: in.Content, apply: func(s string) { raw["content"] = s }}}
	case "Edit":
		return []unit{{text: in.NewString, apply: func(s string) { raw["new_string"] = s }}}
	case "MultiEdit":
		edits, _ := raw["edits"].([]any)
		units := make([]unit, 0, len(in.Edits))
		for i := range in.Edits {
			units = append(units, unit{
				text: in.Edits[i].NewString,
				apply: func(s string) {
					if i >= len(edits) {
						return
					}
					if m, ok := edits[i].(map[string]any); ok {
						m["new_string"] = s
					}
				},
			})
		}
		return units
	}
	return nil
}

func runHook(cmd *cobra.Command, _ []string) error {
	rules, ids, err := selectedRules(hookOnly)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return nil
	}
	out := judge(data, rules, ids)
	if out != "" {
		fmt.Fprint(cmd.OutOrStdout(), out)
	}
	return nil
}

// judge answers a payload with the response to print, or "" to let the write
// through. Every unreadable input answers "": a guard that refuses a write it
// could not parse is worse than no guard.
func judge(data []byte, rules []slopfmt.Rule, ids []string) string {
	var in hookInput
	if json.Unmarshal(data, &in) != nil {
		return ""
	}
	if in.HookEventName != "" && in.HookEventName != "PreToolUse" {
		return ""
	}
	var write writeInput
	if json.Unmarshal(in.ToolInput, &write) != nil {
		return ""
	}
	var raw map[string]any
	if json.Unmarshal(in.ToolInput, &raw) != nil {
		return ""
	}

	var removed []string
	var kept []tombstones.Hit
	var findings []string
	changed := false
	for _, u := range writeUnits(in.ToolName, write, raw) {
		repair := slopfmt.Fix(slopfmt.Request{
			Content:         u.text,
			Path:            write.FilePath,
			Rules:           rules,
			IDs:             ids,
			MaxCommentLines: hookMaxLines,
		})
		removed = append(removed, repair.Removed...)
		kept = append(kept, repair.Kept...)
		for _, f := range repair.Findings {
			findings = append(findings, f.String())
		}
		if repair.Changed {
			u.apply(repair.Text)
			changed = true
		}
	}

	switch {
	case len(kept) > 0 || len(findings) > 0:
		return respond(func(r *hookResponse) {
			r.HookSpecificOutput.PermissionDecision = "deny"
			r.HookSpecificOutput.PermissionDecisionReason = refusal(kept, findings)
		})
	case changed:
		return respond(func(r *hookResponse) {
			r.HookSpecificOutput.UpdatedInput = raw
			r.HookSpecificOutput.AdditionalContext = notice(write.FilePath, removed)
		})
	}
	return ""
}

func respond(fill func(*hookResponse)) string {
	var r hookResponse
	r.HookSpecificOutput.HookEventName = "PreToolUse"
	fill(&r)
	out, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(out)
}

// reportCap bounds what a message carries: a refusal nobody reads stops nothing.
const reportCap = 6

// notice is what the model is told after the fact. The write went through, so
// it names what was cut rather than asking for a retry.
func notice(path string, removed []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "slopfmt repaired this write to %s. It removed:\n", path)
	for _, line := range capped(removed) {
		fmt.Fprintf(&b, "  %q\n", strings.TrimSpace(line))
	}
	b.WriteString("\nThe text that was written no longer carries them. Read the sentence back and make it read naturally.")
	return b.String()
}

// refusal names each finding no rewrite resolved, and the line it sits on.
func refusal(kept []tombstones.Hit, findings []string) string {
	var b strings.Builder
	b.WriteString("blocked: this write carries what no rewrite can repair.\n")
	for _, hit := range capped(hitLines(kept)) {
		b.WriteString("  " + hit + "\n")
	}
	for _, f := range capped(findings) {
		b.WriteString("  " + f + "\n")
	}
	return b.String()
}

func hitLines(kept []tombstones.Hit) []string {
	lines := make([]string, 0, len(kept))
	for _, hit := range kept {
		lines = append(lines, fmt.Sprintf("[%s] %s: %q\n    %s", hit.ID, hit.Tell, hit.Phrase, hit.Line))
	}
	return lines
}

// capped trims a list to what a reader takes in, saying so when it trimmed.
func capped(lines []string) []string {
	if len(lines) <= reportCap {
		return lines
	}
	return append(lines[:reportCap:reportCap], "... and more, not listed")
}
