// Package repairwrite repairs the file a write just landed, in place.
//
// Judging the text before it lands means putting the fragment back where it
// goes because a fence and a table are properties of the whole document. The
// file on disk is already whole, so this reads it and rewrites it.
package repairwrite

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wow-look-at-my/slopfix"
)

// Result is what the launcher prints and exits with.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// writeTools name the tools that put text on disk.
var writeTools = map[string]bool{
	"Write":        true,
	"Edit":         true,
	"MultiEdit":    true,
	"NotebookEdit": true,
}

// input is the part of a PostToolUse payload this reads.
type input struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// Run repairs the written file and names what changed.
//
// The write has already happened, so there is nothing here to refuse and no
// exit code that would undo it. Every payload it cannot read answers nothing.
func Run(r io.Reader) Result {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var in input
	if json.Unmarshal(raw, &in) != nil {
		return Result{}
	}
	if in.HookEventName != "" && in.HookEventName != "PostToolUse" {
		return Result{}
	}
	path := in.ToolInput.FilePath
	if !writeTools[in.ToolName] || path == "" {
		return Result{}
	}

	repair, err := slopfix.FixFile(path)
	if err != nil {
		// Said on stderr rather than swallowed: a repair that never ran, and
		// never said so, reads exactly like a file with nothing to repair.
		return Result{Stderr: fmt.Sprintf("slopfix: did not repair %s: %v\n", path, err)}
	}
	if !repair.Changed {
		return Result{}
	}
	return Result{Stdout: context(notice(path, repair))}
}

// reportCap bounds the list, because a notice nobody reads changes nothing.
const reportCap = 6

func notice(path string, repair slopfix.Repair) string {
	var b strings.Builder
	fmt.Fprintf(&b, "slopfix repaired %s after the write.\n", path)
	for _, line := range capped(repair.Removed) {
		fmt.Fprintf(&b, "  removed %q\n", strings.TrimSpace(line))
	}
	b.WriteString("\nThe file on disk differs from what you wrote. Read it back before you quote it or edit it again.")
	return b.String()
}

// capped trims a list to what a reader takes in, saying so when it trimmed.
func capped(lines []string) []string {
	if len(lines) <= reportCap {
		return lines
	}
	return append(lines[:reportCap:reportCap], "... and more, not listed")
}

// context wraps text in the envelope a hook answers with.
func context(text string) string {
	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": text,
		},
	})
	if err != nil {
		return ""
	}
	return string(out)
}
