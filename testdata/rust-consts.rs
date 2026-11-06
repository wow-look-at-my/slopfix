//! Prompt helpers for the summary under a long thinking block.

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

/// The thinking text worth a summary, cut to the input budget, or `None` when
/// it is too short to need one.
pub(crate) fn summarizable_thinking(thinking: &str) -> Option<String> {
    let thinking = thinking.trim();
    if thinking.len() < THINKING_SUMMARY_MIN_CHARS {
        return None;
    }
    Some(thinking.to_string())
}
