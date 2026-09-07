package workflow

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// MaxCommentLines bounds a run of comment lines, and takes no input.
const MaxCommentLines = 1

// commentBlocks reports each run of comment lines past the limit. A blank line
// neither counts nor ends a run: a reader sees the same paragraph either way.
func commentBlocks(content string) []ste.Finding {
	var out []ste.Finding
	start, end, count := 0, 0, 0

	flush := func() {
		if count > MaxCommentLines {
			out = append(out, ste.Finding{
				Line: start,
				// The block is the finding, so a reader underlines all of it.
				EndLine: end,
				ID:      IDCommentBlock,
				Rule:   fmt.Sprintf("%d comment lines in a row, where the limit is %d", count, MaxCommentLines),
				Detail: span(start, end),
				Fix:    "Shorten this to one line. Say only what a reader needs right here, and put the rest in the commit message.",
			})
		}
		count = 0
	}

	for index, line := range lines(content) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if count == 0 {
				start = index + 1
			}
			count++
			end = index + 1
			continue
		}
		if trimmed == "" {
			continue
		}
		if count > 0 {
			flush()
		}
	}
	if count > 0 {
		flush()
	}
	return out
}

// span names the lines a block covers, for a report that prints one line of text.
func span(start, end int) string {
	if start == end {
		return fmt.Sprintf("line %d", start)
	}
	return fmt.Sprintf("lines %d-%d", start, end)
}
