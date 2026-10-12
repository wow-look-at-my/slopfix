// diff.go renders a repair as a unified diff, so a caller can print exactly
// what a rewrite changed.
package commentfix

import (
	"github.com/pmezard/go-difflib/difflib"
)

// diffContext is how many unchanged lines a hunk keeps around a change.
const diffContext = 2

// UnifiedDiff renders the change from before to after as a unified diff of
// path. It answers "" when nothing changed.
func UnifiedDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	out, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: "a/" + path,
		ToFile:   "b/" + path,
		Context:  diffContext,
	})
	if err != nil {
		panic(err) // it writes to a strings.Builder, which never fails
	}
	return out
}
