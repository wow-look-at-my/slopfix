// handedit.go refuses a comment reworded by hand to clear a check.
//
// slopfix repairs a comment itself, so a reword in a file the rules already
// report answers the check by rephrasing rather than by repair. The wording
// then drifts from what the fixer would have written, and the next run reports
// something else. This asks the file on disk what the edit does, because the
// new text alone cannot say whether the code moved.
package cmd

import (
	"os"
	"strings"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentedit"
)

// edit is a replacement a write performs, as the payload states it.
type edit struct {
	old string
	new string
}

// editsOf reads the replacements the named tool carries. Write replaces the
// whole file and states no replacement, so it has none.
func editsOf(tool string, in writeInput) []edit {
	switch tool {
	case "Edit":
		return []edit{{old: in.OldString, new: in.NewString}}
	case "MultiEdit":
		out := make([]edit, 0, len(in.Edits))
		for _, e := range in.Edits {
			out = append(out, edit{old: e.OldString, new: e.NewString})
		}
		return out
	}
	return nil
}

// handEditReason refuses the write, or answers "" to stay out of the way.
//
// It answers "" on every uncertainty: a file it cannot read, a replacement that
// does not appear exactly once, a language with no grammar, and a file the
// rules read cleanly. Writing a new comment is ordinary work.
func handEditReason(tool string, in writeInput, rules []slopfix.Rule, ids []string) string {
	edits := editsOf(tool, in)
	if len(edits) == 0 || in.FilePath == "" {
		return ""
	}
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return ""
	}
	before := string(data)

	after, ok := applyEdits(before, edits)
	if !ok {
		return ""
	}
	if !commentedit.OnlyComments(in.FilePath, before, after) {
		return ""
	}
	if !commentedit.Reports(findingIDs(before, in.FilePath, rules, ids)) {
		return ""
	}
	return "blocked: this edit rewords a comment and leaves the code as it was, in a file slopfix already reports.\n" +
		commentedit.Remedy + ":\nrun: slopfix fix " + in.FilePath
}

// applyEdits replays the replacements onto the file. A replacement that is
// absent, or that appears more than once, leaves the result unknown, and an
// unknown result is never refused here.
func applyEdits(src string, edits []edit) (string, bool) {
	out := src
	for _, e := range edits {
		if e.old == "" || strings.Count(out, e.old) != 1 {
			return "", false
		}
		out = strings.Replace(out, e.old, e.new, 1)
	}
	return out, true
}

// findingIDs runs the same rules the caller selected over the file as it stands,
// and answers what they report.
func findingIDs(src, path string, rules []slopfix.Rule, ids []string) []string {
	repair := slopfix.Fix(slopfix.Request{
		Content:         src,
		Path:            path,
		Rules:           rules,
		IDs:             ids,
		MaxCommentLines: hookMaxLines,
	})
	out := make([]string, 0, len(repair.Findings))
	for _, f := range repair.Findings {
		out = append(out, f.ID)
	}
	return out
}
