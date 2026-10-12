// lists.go holds counts/inventory-list for the prose of a comment. The repair
// deletes each sentence that lists what the code holds, and each list of such
// members. A comment the repair empties goes whole.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// FixerList is the fixer that deletes an enumeration from a comment.
const FixerList = "comments/inventory-list"

func init() {
	fixer.Register(fixer.Spec{
		Label:    FixerList,
		Families: []string{"counts"},
		Rules:    []string{counts.IDList},
		Files:    []fixer.Kind{fixer.Source, fixer.Workflow},
		Place:    19,
		Repair: func(f *fixer.File) {
			f.ApplyComments(listEdits(f.Path, f.Text(), f.Kind == fixer.Workflow))
		},
	})
}

// ListHit is an enumeration in a comment.
type ListHit struct {
	// Line is the line the enumeration opens on, counted from one.
	Line int
	// Sentence quotes the enumeration.
	Sentence string
}

// CheckLists reports every enumeration in the comments of src.
func CheckLists(filename, src string) []ListHit {
	if IsGenerated(filename, src) {
		return nil
	}
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	lines := strings.Split(src, "\n")
	var hits []ListHit
	for _, p := range paragraphsOf(lines, runs) {
		if !judged(lines, p) {
			continue
		}
		segs := segmentsOf(lines, p)
		dropped := droppedItems(segs)
		for k, seg := range segs {
			line := p.lines[seg.lines[0]] + 1
			if dropped[k] {
				hits = append(hits, ListHit{Line: line, Sentence: seg.prose})
				continue
			}
			for _, c := range counts.ProseCuts(seg.prose) {
				hits = append(hits, ListHit{Line: line, Sentence: c.Detail})
			}
		}
	}
	return hits
}

// droppedItems marks each segment that is an item of a comment list whose
// items open with a member, when at least MinMembers of them do.
func droppedItems(segs []segment) []bool {
	out := make([]bool, len(segs))
	for k := 0; k < len(segs); {
		if !opensListItem(segs[k].prose) {
			k++
			continue
		}
		j, opening := k, 0
		for j < len(segs) && opensListItem(segs[j].prose) {
			if counts.OpensWithMember(itemText(segs[j].prose)) {
				opening++
			}
			j++
		}
		if opening >= counts.MinMembers {
			for i := k; i < j; i++ {
				out[i] = true
			}
		}
		k = j
	}
	return out
}

// itemText is a list item's prose past its marker.
func itemText(prose string) string {
	trimmed := strings.TrimLeft(prose, "-*+0123456789.)")
	return strings.TrimSpace(trimmed)
}

// cutLists answers prose with each sentence that holds an enumeration taken out.
func cutLists(prose string) string {
	cuts := counts.ProseCuts(prose)
	if len(cuts) == 0 {
		return prose
	}
	spans := make([]counts.Span, 0, len(cuts))
	for _, c := range cuts {
		spans = append(spans, c.Span)
	}
	return counts.CutProse(prose, spans)
}

// listEdits answers an edit per comment paragraph that holds an enumeration.
func listEdits(filename, src string, singleLine bool) []edit.Edit {
	if IsGenerated(filename, src) {
		return nil
	}
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	lines := strings.Split(src, "\n")
	var edits []edit.Edit
	for _, p := range paragraphsOf(lines, runs) {
		if !judged(lines, p) {
			continue
		}
		segs := segmentsOf(lines, p)
		dropped := droppedItems(segs)
		drop := map[string]bool{}
		for k, seg := range segs {
			if dropped[k] {
				drop[seg.prose] = true
			}
		}
		rewrite := func(prose string) string {
			if drop[prose] {
				return ""
			}
			return cutLists(prose)
		}
		out, changed := rewriteSegments(lines, p, singleLine || p.code != "", rewrite)
		if !changed {
			continue
		}
		first, last := p.lines[0], p.lines[len(p.lines)-1]
		if p.code != "" && len(out) > 0 && emptyComment(out[1:]) && out[0] == p.code {
			code := strings.TrimRight(p.code, " \t")
			e := edit.Lines(src, first, last, len(code), []string{""})
			edits = append(edits, e)
			continue
		}
		if p.code == "" && emptyComment(out) {
			// A blank comment line beside the deleted paragraph goes with it, so no break is left doubled or dangling.
			switch {
			case last+1 < len(lines) && isBlankComment(lines[last+1]) && strings.TrimSpace(lines[last+1]) != "":
				last++
			case first > 0 && isBlankComment(lines[first-1]) && strings.TrimSpace(lines[first-1]) != "":
				first--
			}
			edits = append(edits, edit.Lines(src, first, last, 0, nil))
			continue
		}
		col := len(p.code)
		out[0] = out[0][min(col, len(out[0])):]
		edits = append(edits, edit.Lines(src, first, last, col, out))
	}
	return edits
}

// emptyComment reports lines that carry no prose.
func emptyComment(lines []string) bool {
	for _, line := range lines {
		if !isBlankComment(line) {
			return false
		}
	}
	return true
}
