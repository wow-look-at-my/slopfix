package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/repairwrite"
	"github.com/wow-look-at-my/slopfix/tombstones"
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
		Short: "Repair a Claude Code write, reading its hook payload on stdin",
		Long: "hook reads a write's payload on stdin and writes the hook's own response\n" +
			"on stdout. It serves both events from one command, deciding on the event\n" +
			"the payload names, so a manifest never carries a command per event.\n\n" +
			"On PostToolUse it repairs the file the write landed, in place and whole. A\n" +
			"fence, a table and a comment block are properties of a file rather than of\n" +
			"a fragment, and the file on disk is the only place all three are true.\n\n" +
			"On PreToolUse it repairs the text a write adds. It never refuses one: what\n" +
			"no rewrite repairs is named in the context the model reads, and the write\n" +
			"still lands.\n\n" +
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
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	Edits     []struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	} `json:"edits"`
}

// hookResponse carries a repaired payload, a notice, or both. Allowing an
// untouched write prints nothing at all.
type hookResponse struct {
	HookSpecificOutput struct {
		HookEventName     string         `json:"hookEventName"`
		UpdatedInput      map[string]any `json:"updatedInput,omitempty"`
		AdditionalContext string         `json:"additionalContext,omitempty"`
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

	// A single command serves both halves of a write. The event decides
	// which, because a manifest that names a command per event is a list to
	// keep in step with this.
	var event struct {
		HookEventName string `json:"hook_event_name"`
	}
	if json.Unmarshal(data, &event) == nil && event.HookEventName == "PostToolUse" {
		res := repairwrite.Run(bytes.NewReader(data))
		if res.Stdout != "" {
			fmt.Fprint(cmd.OutOrStdout(), res.Stdout)
		}
		if res.Stderr != "" {
			fmt.Fprint(cmd.ErrOrStderr(), res.Stderr)
		}
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
func judge(data []byte, rules []slopfix.Rule, ids []string) string {
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
		repair := slopfix.Fix(slopfix.Request{
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

	if !changed && len(kept) == 0 && len(findings) == 0 {
		return ""
	}
	return respond(func(r *hookResponse) {
		if changed {
			r.HookSpecificOutput.UpdatedInput = raw
		}
		r.HookSpecificOutput.AdditionalContext = notice(write.FilePath, removed, kept, findings)
	})
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

// notice is what the model is told after the fact. The write went through
// either way, so it names what was cut and what was left rather than asking
// for a retry.
func notice(path string, removed []string, kept []tombstones.Hit, findings []string) string {
	var b strings.Builder
	if len(removed) > 0 {
		fmt.Fprintf(&b, "slopfix repaired this write to %s. It removed:\n", path)
		for _, line := range capped(removed) {
			fmt.Fprintf(&b, "  %q\n", strings.TrimSpace(line))
		}
		b.WriteString("\nThe text that was written no longer carries them. Read the sentence back and make it read naturally.\n")
	}
	rest := append(hitLines(kept), findings...)
	if len(rest) == 0 {
		return b.String()
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "slopfix left these in %s, because no rewrite repairs them:\n", path)
	for _, line := range capped(rest) {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\nThe write went through as it stands. Reword them in a follow-up edit.")
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
