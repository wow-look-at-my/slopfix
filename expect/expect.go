// Package expect reads slopfix-expect annotations and checks a repair against them.
//
// A line carrying `slopfix-expect: "text"` states what slopfix turns that line
// into, as a Go-quoted string; "" states that the repair deletes it. A file
// carrying any annotation is a fixture: it is never rewritten, and every line
// the repair would change has to be annotated with exactly what it would write.
package expect

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Marker opens an annotation.
const Marker = "slopfix-expect:"

// openers are the comment markers an annotation may stand behind, longest first.
var openers = []string{"<!--", "/*", "//", "--", "#", ";"}

// closers end a block comment the annotation sits in.
var closers = []string{"-->", "*/"}

// Annotations are the expectations a text carries, and the text without them.
type Annotations struct {
	// Stripped is the text with every annotation cut out, which is what the rules judge.
	Stripped string
	// Want maps a line index to the text slopfix must turn that line into.
	Want map[int]string
	// cuts are the byte spans removed from the text, in order.
	cuts []span
	// Errors name each annotation that does not parse.
	Errors []string
}

type span struct{ start, end int }

// Parse finds every annotation in text.
func Parse(text string) Annotations {
	out := Annotations{Want: map[int]string{}}
	var b strings.Builder
	offset := 0
	for i, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		at := strings.Index(body, Marker)
		start, ok := annotationStart(body, at)
		if !ok {
			b.WriteString(line)
			offset += len(line)
			continue
		}
		raw := strings.TrimSpace(body[at+len(Marker):])
		for _, c := range closers {
			raw = strings.TrimSpace(strings.TrimSuffix(raw, c))
		}
		want, err := strconv.Unquote(raw)
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("line %d: %s takes a Go-quoted string, got %s", i+1, Marker, raw))
		}
		out.Want[i] = want
		b.WriteString(body[:start])
		b.WriteString(line[len(body):])
		out.cuts = append(out.cuts, span{offset + start, offset + len(body)})
		offset += len(line)
	}
	out.Stripped = b.String()
	return out
}

// annotationStart answers where the annotation begins, at the blanks before
// its comment opener. A marker with no opener before it is text, not an annotation.
func annotationStart(body string, at int) (int, bool) {
	if at < 0 {
		return 0, false
	}
	start := len(strings.TrimRight(body[:at], " \t"))
	for _, o := range openers {
		if strings.HasSuffix(body[:start], o) {
			return len(strings.TrimRight(body[:start-len(o)], " \t")), true
		}
	}
	return 0, false
}

// Any reports whether the text carried an annotation.
func (a Annotations) Any() bool { return len(a.Want) > 0 || len(a.Errors) > 0 }

// Shift maps a byte offset in the annotated text to the same place in Stripped.
func (a Annotations) Shift(offset int) int {
	removed := 0
	for _, c := range a.cuts {
		if c.start >= offset {
			break
		}
		removed += min(offset, c.end) - c.start
	}
	return offset - removed
}

// Check compares what the repair made of Stripped against the annotations,
// and answers every disagreement. An empty answer is a fixture that holds.
func (a Annotations) Check(repaired string) []string {
	failures := append([]string{}, a.Errors...)
	before := strings.Split(a.Stripped, "\n")
	after := strings.Split(repaired, "\n")
	got := map[int]string{}
	matcher := difflib.NewMatcherWithJunk(before, after, false, nil)
	for _, op := range matcher.GetOpCodes() {
		switch op.Tag {
		case 'e':
			continue
		case 'i':
			line := max(op.I1-1, 0)
			text, seen := got[line]
			if !seen {
				text = before[line]
			}
			added := strings.Join(after[op.J1:op.J2], "\n")
			if op.I1 == 0 {
				got[line] = added + "\n" + text
			} else {
				got[line] = text + "\n" + added
			}
		case 'r':
			if op.I2-op.I1 != op.J2-op.J1 {
				got[op.I1] = strings.Join(after[op.J1:op.J2], "\n")
				for i := op.I1 + 1; i < op.I2; i++ {
					got[i] = ""
				}
				continue
			}
			for k := range op.I2 - op.I1 {
				got[op.I1+k] = after[op.J1+k]
			}
		default:
			got[op.I1] = strings.Join(after[op.J1:op.J2], "\n")
			for i := op.I1 + 1; i < op.I2; i++ {
				got[i] = ""
			}
		}
	}
	for i := range before {
		want, annotated := a.Want[i]
		text, changed := got[i]
		switch {
		case annotated && !changed:
			failures = append(failures, fmt.Sprintf("line %d: slopfix made no change to a line annotated %s %q", i+1, Marker, want))
		case !annotated && changed:
			failures = append(failures, fmt.Sprintf("line %d: slopfix changed it to %q, and it carries no %s", i+1, text, Marker))
		case annotated && text != want:
			failures = append(failures, fmt.Sprintf("line %d: slopfix wrote %q, and %s says %q", i+1, text, Marker, want))
		}
	}
	return failures
}
