// Package commentedit refuses a comment rewritten by hand to clear a check.
//
// slopfix repairs a comment itself. `fix` joins a wrapped paragraph, cuts a
// cardinal, expands a contraction and turns a semicolon into the period it
// stands in for. So a comment reworded by hand, in a file the rules already
// report, is a check answered by rephrasing rather than by repair. The wording
// drifts away from what the fixer would have written, and the next run reports
// something else.
//
// The refusal is narrow on purpose. Writing a NEW comment is ordinary work, and
// a file the rules read cleanly is not this package's business. What it catches
// is the edit that leaves the code untouched, in a file already reported.
package commentedit

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/code"
)

// ID names this rule, on a report and on the command line alike.
const ID = "comments/handedit"

// Remedy is what the refusal asks for instead.
const Remedy = "slopfix owns the wording of a comment it reports. Run `slopfix fix` on the file and take what it writes"

// Reports answers whether the file carries a finding a comment edit could
// clear. A caller passes the IDs its own run produced.
func Reports(ids []string) bool {
	for _, id := range ids {
		if strings.HasPrefix(id, "comments/") || strings.HasPrefix(id, "ste/") {
			return true
		}
	}
	return false
}

// OnlyComments answers whether an edit leaves every byte outside a comment
// exactly as it was.
func OnlyComments(path, before, after string) bool {
	if before == after {
		return false
	}
	if !code.Parsed(path) {
		return false
	}
	return blanked(path, before) == blanked(path, after)
}

// blanked returns the file with every comment blanked, so what remains is what
// the compiler reads. A blank of the same length keeps every later offset in
// place.
func blanked(path, src string) string {
	comments := code.Comments(path, src)
	if len(comments) == 0 {
		return src
	}
	var b strings.Builder
	at := 0
	for _, c := range comments {
		if c.Offset < at || c.Offset+len(c.Text) > len(src) {
			continue
		}
		b.WriteString(src[at:c.Offset])
		b.WriteString(strings.Repeat(" ", len(c.Text)))
		at = c.Offset + len(c.Text)
	}
	b.WriteString(src[at:])
	return b.String()
}
