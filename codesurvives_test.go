package slopfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// rustConsts is the file a Write lost consts from. Each const has its own
// doc comment, under a module doc comment.
const rustConsts = `//! Prompt helpers for the summary under a long thinking block.

use crate::sampling::ConversationResponse;
use crate::session::helpers::chat::floor_char_boundary;

/// Thinking shorter than this reads fast enough on its own.
pub(crate) const THINKING_SUMMARY_MIN_CHARS: usize = 800;

/// Safety cap on the summary. The instruction asks for two sentences at most.
pub(crate) const THINKING_SUMMARY_MAX_CHARS: usize = 320;

/// The head and the tail of very long thinking go to the model. The tail
/// holds the conclusion, so it gets the larger share.
const INPUT_HEAD_CHARS: usize = 16_000;
const INPUT_TAIL_CHARS: usize = 32_000;

/// The thinking text worth a summary, cut to the input budget, or ` + "`None`" + ` when
/// it is too short to need one.
pub(crate) fn summarizable_thinking(thinking: &str) -> Option<String> {
    let thinking = thinking.trim();
    if thinking.len() < THINKING_SUMMARY_MIN_CHARS {
        return None;
    }
    Some(thinking.to_string())
}
`

// codeLines is every line that is neither blank nor a comment.
func codeLines(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// A repair rewrites comments. It never deletes the code they document.
func TestARepairNeverDeletesACodeLine(t *testing.T) {
	got := Fix(Request{Content: rustConsts, Path: "a.rs", MaxCommentLines: 14})
	assert.Equal(t, codeLines(rustConsts), codeLines(got.Text), "removed %q", got.Removed)
}
