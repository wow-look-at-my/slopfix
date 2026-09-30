package slopfix

import (
	"fmt"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/wow-look-at-my/slopfix/expect"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
)

// pending answers a finding for each run of lines a fixer would change in the
// text as written. A report of the residue alone passes a file the repair rewrites.
func pending(req Request) []ste.Finding {
	// A fixture is judged against its annotations, not against the repair.
	if expect.Parse(req.Content).Any() {
		return nil
	}
	kind := kindOf(req.Path, req.Content)
	var out []ste.Finding
	for _, fx := range fixer.For(kind) {
		f := openFile(req, kind)
		fixer.Run(f, []fixer.Fixer{fx})
		if f.Text() == req.Content {
			continue
		}
		id := keptID(fx, f)
		for _, line := range changedLines(req.Content, f.Text()) {
			out = append(out, ste.Finding{
				Line: line,
				ID:   id,
				Rule: fmt.Sprintf("the %s repair rewrites this text", fx.Name()),
				Fix:  "Run slopfix fix on the file",
			})
		}
	}
	return out
}

// keptID answers the first rule of fx that the caller's selection keeps.
func keptID(fx fixer.Fixer, f *fixer.File) string {
	for _, id := range fx.IDs() {
		if f.Keeps(id) {
			return id
		}
	}
	return fx.Name()
}

// changedLines answers the first line, counting from the top, of each run of
// lines that differs between before and after.
func changedLines(before, after string) []int {
	matcher := difflib.NewMatcher(difflib.SplitLines(before), difflib.SplitLines(after))
	var lines []int
	for _, op := range matcher.GetOpCodes() {
		if op.Tag != 'e' {
			lines = append(lines, op.I1+1)
		}
	}
	return lines
}
