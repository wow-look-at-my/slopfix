// diff.go renders a repair as a unified diff, so a caller can print exactly
// what a rewrite changed.
package commentfix

import (
	"fmt"
	"strings"
)

// diffContext is how many unchanged lines a hunk keeps around a change.
const diffContext = 2

// UnifiedDiff renders the change from before to after as a unified diff of
// path. It answers "" when nothing changed.
func UnifiedDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	ops := lineOps(a, b)
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n", path, path)
	for _, h := range hunks(ops) {
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", h.aStart+1, h.aLen, h.bStart+1, h.bLen)
		for _, op := range ops[h.from:h.to] {
			out.WriteString(string(op.kind) + op.text + "\n")
		}
	}
	return out.String()
}

// lineOp is a single line of the edit script: kept, removed or added.
type lineOp struct {
	kind byte
	text string
	// a and b are the line indexes this op sits at in each side.
	a, b int
}

// lineOps answers the shortest edit script from a to b, by longest common
// subsequence. A comment repair touches few lines of a file, and the table
// costs len(a) times len(b), which a source file keeps small.
func lineOps(a, b []string) []lineOp {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []lineOp
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ops = append(ops, lineOp{' ', a[i], i, j})
			i, j = i+1, j+1
		case i < len(a) && (j == len(b) || lcs[i+1][j] >= lcs[i][j+1]):
			// A removal prints before the addition that replaces it.
			ops = append(ops, lineOp{'-', a[i], i, j})
			i++
		default:
			ops = append(ops, lineOp{'+', b[j], i, j})
			j++
		}
	}
	return ops
}

// hunk is a run of ops to print, and the lines it spans on each side.
type hunk struct {
	from, to       int
	aStart, aLen   int
	bStart, bLen   int
}

// hunks groups the changed ops with their context, merging groups whose
// context overlaps.
func hunks(ops []lineOp) []hunk {
	var out []hunk
	for i := 0; i < len(ops); i++ {
		if ops[i].kind == ' ' {
			continue
		}
		from := max(0, i-diffContext)
		to := i
		for to < len(ops) {
			if ops[to].kind != ' ' {
				to++
				continue
			}
			// A run of kept lines longer than both contexts ends the hunk.
			run := to
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run == len(ops) || run-to > 2*diffContext {
				to = min(len(ops), to+diffContext)
				break
			}
			to = run
		}
		h := hunk{from: from, to: to, aStart: ops[from].a, bStart: ops[from].b}
		for _, op := range ops[from:to] {
			if op.kind != '+' {
				h.aLen++
			}
			if op.kind != '-' {
				h.bLen++
			}
		}
		out = append(out, h)
		i = to - 1
	}
	return out
}
