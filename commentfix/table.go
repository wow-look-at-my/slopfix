// table.go applies the English the number repair writes instead of a number.
//
// The table itself is rules/, embedded and loaded a single time. Adding a
// phrase stays an edit to XML that needs no Go.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// numbersTable is what rules/ says for="numbers", in file name order.
var numbersTable = table.MustLoad(rules.FS, "numbers")

// Reword rewrites a line of comment prose, applying every table entry to the
// runs outside its quotations. It says nothing about what is left.
func Reword(prose string) string {
	out, _ := rewordChecked(prose)
	return out
}

// rewordChecked is Reword, and it also answers each rewrite the guard threw
// away. Path and Line of a rejection are the caller's to fill.
func rewordChecked(prose string) (string, []Rejection) {
	spans := cardinal.QuotedSpans(prose)
	if len(spans) == 0 {
		return rewordRun(prose)
	}
	var out strings.Builder
	var rejected []Rejection
	outside := func(run string) {
		said, r := rewordOutside(run)
		out.WriteString(said)
		rejected = append(rejected, r...)
	}
	at := 0
	for _, span := range spans {
		outside(prose[at:span.Start])
		out.WriteString(prose[span.Start:span.End])
		at = span.End
	}
	outside(prose[at:])
	return out.String(), rejected
}

// rewordOutside rewrites a run between quotations, keeping the blank that
// borders it. The table collapses runs of whitespace, and the space beside a
// quotation is what holds it apart from the words either side.
func rewordOutside(run string) (string, []Rejection) {
	body := strings.TrimLeft(run, " \t")
	lead := run[:len(run)-len(body)]
	core := strings.TrimRight(body, " \t")
	if core == "" {
		return run, nil
	}
	said, rejected := rewordRun(core)
	return lead + said + body[len(core):], rejected
}

// rewordRun applies every table entry to a run of prose carrying no quotation.
//
// The order is rewrites, then shapes, then patterns. A phrase swap settles the
// idioms earliest. Each entry is a step, and a step the guard refuses leaves
// the prose as the step found it.
func rewordRun(prose string) (string, []Rejection) {
	original := prose
	settle := func(s string) string {
		return capitalise(original, closeDanglingSpace(strings.Join(strings.Fields(s), " ")))
	}
	var rejected []Rejection
	step := func(rule, next string) {
		if next == prose {
			return
		}
		if defect := defectOf(settle(prose), settle(next)); defect != "" {
			rejected = append(rejected, Rejection{Rule: rule, Defect: defect, Wrote: settle(next)})
			return
		}
		prose = next
	}
	for _, r := range numbersTable.Rewrites {
		step(r.ID, replaceWord(prose, r.From, r.To))
	}
	lex := numbersTable.Lexicon()
	for _, e := range numbersTable.Rephrasings {
		step(e.ID, table.Rephrasings(lex, numbersTable.Normals, []table.Rephrase{e}, prose))
	}
	for _, p := range numbersTable.Patterns {
		step(p.ID, p.Replace(prose))
	}
	return settle(prose), rejected
}

// closeDanglingSpace removes the space a deletion leaves before punctuation
// that CLOSES something.
func closeDanglingSpace(prose string) string {
	var out strings.Builder
	for i := 0; i < len(prose); i++ {
		if !isSpaceByte(prose[i]) {
			out.WriteByte(prose[i])
			continue
		}
		next := i
		for next < len(prose) && isSpaceByte(prose[next]) {
			next++
		}
		if next < len(prose) && (prose[next] == '.' || prose[next] == ',') &&
			(next+1 >= len(prose) || isSpaceByte(prose[next+1])) {
			i = next - 1
			continue
		}
		out.WriteByte(prose[i])
	}
	return out.String()
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// replaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone. The replacement takes the case of
// what it stands in for.
func replaceWord(s, word, with string) string {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !wordBoundary(s, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(table.MatchCase(s[at:end], with))
		i = end
	}
	return b.String()
}

// wordBoundary reports whether s[at:end] stands as its own word. A marker
// joining it to a name spells a name that does not exist.
func wordBoundary(s string, at, end int) bool {
	if at > 0 && (isWordByte(s[at-1]) || joinsName(s, at-1, -1)) {
		return false
	}
	return end >= len(s) || !(isWordByte(s[end]) || joinsName(s, end, +1))
}

// nameMarkers join an identifier, an import path or a label into a name.
const nameMarkers = "._/:"

// joinsName reports whether s[at] is a marker with a name character beyond it,
// reading in the given direction.
func joinsName(s string, at, step int) bool {
	if strings.IndexByte(nameMarkers, s[at]) < 0 {
		return false
	}
	beyond := at + step
	return beyond >= 0 && beyond < len(s) && isWordByte(s[beyond])
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalise gives the repaired line the opening case the source line had. A
// comment line often continues a wrapped sentence rather than opening it, so
// the case the author wrote is the only reliable answer.
func capitalise(original, s string) string {
	if s == "" || original == "" {
		return s
	}
	upper := original[0] >= 'A' && original[0] <= 'Z'
	switch c := s[0]; {
	case upper && c >= 'a' && c <= 'z':
		return string(c-32) + s[1:]
	case !upper && c >= 'A' && c <= 'Z':
		return string(c+32) + s[1:]
	}
	return s
}
