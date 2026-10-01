// edit.go is the door a repair writes a document through.
//
// An edit must sit inside a single prose block, as the CommonMark parser found
// it. After the splice the document is parsed again, and every verbatim block
// must come back as it was, in the same order. Text that opens a fence, starts
// a heading or ends a list changes that answer, so the edit that wrote it
// never lands.
package markdown

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
)

// Apply writes the edits into content through the gate. An edit outside the
// scope is dropped without a finding. An edit that leaves its prose block, or
// that changes a verbatim block once written, is refused.
func Apply(content string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(content, scope)
	}
	spans := proseSpans(content)
	want := verbatim(content)
	return edit.Gate(content, edits, scope,
		func(e edit.Edit) string {
			for _, s := range spans {
				if e.Start >= s[0] && e.End <= s[1] {
					return ""
				}
			}
			return "it reaches past its prose block"
		},
		func(text string) bool { return sameLines(verbatim(text), want) })
}

// BlockEdit is an edit that replaces a whole prose block with lines.
func BlockEdit(content string, b Block, lines []string) edit.Edit {
	return edit.Rows(content, b.Start-1, b.Start-2+len(b.Lines), 0, lines)
}

// proseSpans answers the bytes each prose block covers, line ends inside it
// included and the last one left out.
func proseSpans(content string) [][2]int {
	var out [][2]int
	for _, b := range Split(content) {
		if b.Kind != Prose {
			continue
		}
		e := BlockEdit(content, b, []string{""})
		out = append(out, [2]int{e.Start, e.End})
	}
	return out
}

// verbatim answers every line the parser keeps as written, in order.
func verbatim(content string) []string {
	var out []string
	for _, b := range Split(content) {
		if b.Kind == Verbatim {
			out = append(out, strings.Join(b.Lines, "\n"))
		}
	}
	return out
}

func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
