package workflow

import (
	"fmt"
	"regexp"
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
	body := blockScalarBody(rows)
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

// scalarHeader ends a line that opens a literal or folded block scalar, with
// the optional indentation and chomping indicators YAML allows after it.
var scalarHeader = regexp.MustCompile(`(^|\s)[|>][0-9]*[+-]?(\s+#.*)?\s*$`)

// blockScalarBody reports, per row, whether it sits inside a block scalar. The
// body is every row indented past the header's own indentation, which is what
// ends it: YAML reads the rest of the document from the first row that is not.
func blockScalarBody(rows []string) []bool {
	inside := make([]bool, len(rows))
	for i := range rows {
		if inside[i] || !scalarHeader.MatchString(rows[i]) {
			continue
		}
		open := indentOf(rows[i])
		for j := i + 1; j < len(rows); j++ {
			if strings.TrimSpace(rows[j]) == "" {
				inside[j] = true
				continue
			}
			if indentOf(rows[j]) <= open {
				break
			}
			inside[j] = true
		}
	}
	return inside
}

// indentOf measures the whitespace a row opens with.
func indentOf(row string) int {
	return len(row) - len(strings.TrimLeft(row, " \t"))
}

// span names the lines a block covers, for a report that prints plain text.
func span(start, end int) string {
	if start == end {
		return fmt.Sprintf("line %d", start)
	}
	return fmt.Sprintf("lines %d-%d", start, end)
}
