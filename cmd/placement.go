// placement.go judges a write where it LANDS rather than on its own.
//
// An edit's new text carries no file around it. A comment in it documents
// nothing, because the declaration it sits above is not in the fragment. A
// fenced block's lines read as a hand-wrapped paragraph for the same reason.
// Judged alone, an ordinary edit is refused for what the file supplies.
//
// So the fragment is put back first: the file is read, the edit replayed, and
// the rules run over the whole result. What the file already carried is then
// subtracted, because a finding the write did not introduce belongs to whoever
// wrote it. A finding is matched by rule and message rather than by line, since
// every line below an edit moves.
package cmd

import (
	"os"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// placed is what an edit introduces, once the file supplies its surroundings.
type placed struct {
	// findings are the rules the write's own text broke.
	findings []ste.Finding
	// ok is false when the edit could not be pinned to the file, and the
	// caller then judges the fragment as it always did.
	ok bool
}

// place replays the edits onto the file and answers what the write introduced.
func place(tool string, in writeInput, rules []slopfix.Rule, ids []string) placed {
	edits := editsOf(tool, in)
	if len(edits) == 0 || in.FilePath == "" {
		return placed{}
	}
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return placed{}
	}
	before := string(data)
	after, ok := applyEdits(before, edits)
	if !ok {
		return placed{}
	}
	return placed{findings: introduced(before, after, in.FilePath, rules, ids), ok: true}
}

// introduced subtracts what the file already carried. Each earlier finding
// cancels a matching one, so a second copy of a sentence the file already
// breaks is still the write's own.
func introduced(before, after, path string, rules []slopfix.Rule, ids []string) []ste.Finding {
	had := map[string]int{}
	for _, f := range findingsOver(before, path, rules, ids) {
		had[f.ID+"\x00"+f.Message]++
	}
	var out []ste.Finding
	for _, f := range findingsOver(after, path, rules, ids) {
		key := f.ID + "\x00" + f.Detail
		if had[key] > 0 {
			had[key]--
			continue
		}
		out = append(out, f)
	}
	return out
}

// findingsOver runs the caller's own rule selection over a whole document.
func findingsOver(src, path string, rules []slopfix.Rule, ids []string) []ste.Finding {
	return slopfix.Fix(slopfix.Request{
		Content:         src,
		Path:            path,
		Rules:           rules,
		IDs:             ids,
		MaxCommentLines: hookMaxLines,
	}).Findings
}
