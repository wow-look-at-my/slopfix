package workflow

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// MaxCommentLines bounds a run of comment lines, and takes no input.
const MaxCommentLines = 1

// commentBlocks reports each run of comment lines past the limit. A blank line
// neither counts nor ends a run.
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
				Rule:    fmt.Sprintf("%d comment lines in a row, where the limit is %d", count, MaxCommentLines),
				Detail:  span(start, end),
				Fix:     "Shorten this to one line. Say only what a reader needs right here, and put the rest in the commit message.",
			})
		}
		count = 0
	}

	rows := lines(content)
	body := blockScalarRows(content)
	for index, line := range rows {
		trimmed := strings.TrimSpace(line)
		// A # inside a block scalar opens a shell comment in a script, which
		// this rule has nothing to say about and the repair must not fold.
		if body[index] {
			if count > 0 {
				flush()
			}
			continue
		}
		// An aligned row is a table or an example, so it is data: it neither
		// counts nor joins, and the prose either side of it are separate runs.
		if aligned(trimmed) {
			if count > 0 {
				flush()
			}
			continue
		}
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

// aligned reports whether a comment row lays its words out in columns: its
// text is indented past the blank after the marker, or a run of blanks that
// no sentence end explains sits between its words. Joining such a row into
// prose destroys what the layout says.
func aligned(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	text := strings.TrimPrefix(strings.TrimPrefix(trimmed, "#"), " ")
	if strings.TrimSpace(text) == "" {
		return false
	}
	if text[0] == ' ' || text[0] == '\t' || strings.Contains(text, "\t") {
		return true
	}
	for i := 1; i+1 < len(text); i++ {
		if text[i] == ' ' && text[i+1] == ' ' && !strings.ContainsRune(".!?:", rune(text[i-1])) {
			return true
		}
	}
	return false
}

// span names the lines a block covers, for a report that prints plain text.
func span(start, end int) string {
	if start == end {
		return fmt.Sprintf("line %d", start)
	}
	return fmt.Sprintf("lines %d-%d", start, end)
}
