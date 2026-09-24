// placement.go judges a write where it LANDS rather than on its own.
//
// An edit's new text carries no file around it. A comment in it documents
// nothing, because the declaration it sits above is not in the fragment. A
// fenced block's lines read as a hand-wrapped paragraph for the same reason.
// Judged alone, an ordinary edit is flagged for what the file supplies.
//
// So the fragment is put back earliest: the file is read, the edit replayed,
// and the rules run over the whole result. What the file already carried is
// then subtracted, because a finding the write did not introduce belongs to
// whoever wrote it. A finding is matched by rule and message rather than by
// line, since every line below an edit moves.
package cmd

import (
	"os"
	"strings"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
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

// applyEdits replays the replacements onto the file. A replacement that is
// absent, or that appears more than a single time, leaves the result unknown,
// and the caller then judges the fragment as it always did.
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

// placed is what an edit introduces, a single time the file supplies its surroundings.
type placed struct {
	// findings are the rules the write's own text broke.
	findings []ste.Finding
	// repair is the whole-file repair, cut back to the edit.
	repair *slopfix.Repair
	// ok is false when the edit could not be pinned to the file, and the caller then judges the fragment as it always did.
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
	fixed := fixOver(after, in.FilePath, rules, ids)
	p := placed{findings: introduced(fixOver(before, in.FilePath, rules, ids).Findings, fixed.Findings), ok: true}
	// A MultiEdit's spans cannot be told apart a single time the repair has moved them.
	if len(edits) == 1 {
		p.repair = spliceBack(before, edits[0], fixed)
	}
	return p
}

// spliceBack cuts a whole-file repair back to the text the edit replaces. It
// answers nil when the repair changed anything outside the edit, because that
// text belongs to whoever wrote it.
func spliceBack(before string, e edit, fixed slopfix.Repair) *slopfix.Repair {
	at := strings.Index(before, e.old)
	prefix, suffix := before[:at], before[at+len(e.old):]
	text := fixed.Text
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) ||
		len(text) < len(prefix)+len(suffix) {
		return nil
	}
	out := fixed
	out.Text = text[len(prefix) : len(text)-len(suffix)]
	out.Changed = out.Text != e.new
	out.Kept = nil
	for _, hit := range fixed.Kept {
		if strings.Contains(out.Text, hit.Phrase) {
			out.Kept = append(out.Kept, hit)
		}
	}
	return &out
}

// introduced subtracts what the file already carried.
func introduced(before, after []ste.Finding) []ste.Finding {
	had := map[string]int{}
	for _, f := range before {
		had[f.ID+"\x00"+f.Detail]++
	}
	var out []ste.Finding
	for _, f := range after {
		key := f.ID + "\x00" + f.Detail
		if had[key] > 0 {
			had[key]--
			continue
		}
		out = append(out, f)
	}
	return out
}

// fixOver runs the caller's own rule selection over a whole document.
func fixOver(src, path string, rules []slopfix.Rule, ids []string) slopfix.Repair {
	return slopfix.Fix(slopfix.Request{
		Content:         src,
		Path:            path,
		Rules:           rules,
		IDs:             ids,
		MaxCommentLines: hookMaxLines,
	})
}
