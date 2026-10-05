// forkscope.go keeps a fork's run to the lines the fork wrote itself.
//
// The forkscope package finds those lines. A tree run skips every file the fork
// never touched. In a file it did touch, a repair lands only on the fork's
// lines, and a finding counts only when it names one.
package slopfix

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// FileScope answers the lines of content that the fork wrote, for content
// headed for path, or nil when path is in no fork. A fork whose base cannot be
// read is an error.
func FileScope(r forkscope.Resolver, path, content string) (*forkscope.Scope, error) {
	base, err := BaseOf(r, path)
	if err != nil || base == nil {
		return nil, err
	}
	return base.File(path, content)
}

// BaseOf answers the fork base of the work tree path is in, or nil when it is
// in no fork. A path that does not exist yet is in the work tree of its
// nearest directory that does.
func BaseOf(r forkscope.Resolver, path string) (*forkscope.Base, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return r.Base(existingDir(filepath.Dir(abs)))
}

// existingDir answers dir, or its nearest parent that exists.
func existingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// FixFileIn is FixFileWith in a fork. A file the fork never touched is neither
// read nor written, and a repair lands only on the lines the fork wrote.
func FixFileIn(r forkscope.Resolver, path string, req Request) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	scope, err := FileScope(r, path, string(content))
	if err != nil {
		return Repair{}, err
	}
	if scope != nil && scope.Empty() {
		return Repair{Text: string(content)}, nil
	}
	req.Owned = scope
	return FixFileWith(path, req)
}

// within keeps a repair of req to the lines req.Owned names. A change to any
// other line goes back as req.Content had it. The findings and the tombstones
// then come from the text that lands, and only those on owned lines stay.
func within(req Request, repair Repair) Repair {
	scope := req.Owned
	if scope == nil || scope.All() {
		return repair
	}
	text := forkscope.Keep(req.Content, repair.Text, scope)
	if text != repair.Text {
		if repair.Scope.Bounded && repair.Scope.Start <= repair.Scope.End {
			repair.Scope.End = len(text) - (len(repair.Text) - repair.Scope.End)
		}
		again := req
		again.Content, again.Owned = text, nil
		again.Scope = edit.Nowhere()
		landed := Fix(again)
		repair.Findings, repair.Kept = landed.Findings, landed.Kept
		repair.Removed = landedRemovals(req.Content, text, repair.Removed)
		repair.Text, repair.Changed = text, text != req.Content
		if !repair.Changed {
			repair.Rewrites = 0
		}
	}
	repair = alone(req, repair)
	repair = fold(req, repair)
	owned := forkscope.Carry(req.Content, repair.Text, scope)
	repair.Findings = ownedFindings(repair.Findings, owned)
	repair.Kept = ownedHits(repair.Kept, owned)
	return upstreamRuns(req.Owned, repair.Text, repair)
}

// blockRun is the rows, counted from one, that a block finding judges.
type blockRun struct{ first, last int }

// ownedRuns answers each run a block rule reports on a line the fork wrote.
func ownedRuns(req Request, repair Repair) []blockRun {
	owned := forkscope.Carry(req.Content, repair.Text, req.Owned)
	var runs []blockRun
	for _, f := range repair.Findings {
		if blockRules.Contains(f.ID) && owned.Holds(f.Line, max(f.Line, f.EndLine)) {
			runs = append(runs, blockRun{f.Line, max(f.Line, f.EndLine)})
		}
	}
	for _, h := range repair.Kept {
		if blockRules.Contains(h.ID) && h.LineNo > 0 && owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			runs = append(runs, blockRun{h.LineNo, max(h.LineNo, h.EndLineNo)})
		}
	}
	return runs
}

// blockRules judge a run of lines whole.
var blockRules = set.Of(workflow.IDCommentBlock, tombstones.IDVolume)

// alone runs each block rule still reporting on a fork line by itself, and
// keeps what lands on the fork's lines. Run with every rule, a repair of an
// upstream line beside the run joins the run's change in one diff hunk. The
// whole hunk goes back.
func alone(req Request, repair Repair) Repair {
	owned := forkscope.Carry(req.Content, repair.Text, req.Owned)
	ids := set.New[string]()
	for _, f := range repair.Findings {
		if blockRules.Contains(f.ID) && owned.Holds(f.Line, max(f.Line, f.EndLine)) {
			ids.Add(f.ID)
		}
	}
	for _, h := range repair.Kept {
		if blockRules.Contains(h.ID) && h.LineNo > 0 && owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			ids.Add(h.ID)
		}
	}
	for _, id := range slices.Sorted(ids.All()) {
		one := req
		one.Content, one.Scope, one.Owned, one.IDs = repair.Text, repair.Scope, nil, []string{id}
		fixed := fixAll(one)
		owned = forkscope.Carry(req.Content, repair.Text, req.Owned)
		text := forkscope.Keep(repair.Text, fixed.Text, owned)
		if text == repair.Text {
			continue
		}
		again := req
		again.Content, again.Owned, again.Scope = text, nil, edit.Nowhere()
		landed := Fix(again)
		repair.Findings, repair.Kept = landed.Findings, landed.Kept
		repair.Removed = append(repair.Removed, landedRemovals(repair.Text, text, fixed.Removed)...)
		if repair.Scope.Bounded && repair.Scope.Start <= repair.Scope.End {
			repair.Scope.End = len(text) - (len(repair.Text) - repair.Scope.End)
		}
		repair.Text, repair.Changed = text, text != req.Content
	}
	return repair
}

// fold repairs each run a block rule still reports on a line the fork wrote,
// where the fork's changes made the run longer than the base had it. The
// lines each such change added join the last line it kept, so the change
// takes no more lines than. The edits pass the gate that proves they changed
// only comment.
func fold(req Request, repair Repair) Repair {
	runs := ownedRuns(req, repair)
	if len(runs) == 0 {
		return repair
	}
	// With no base to measure from, the finding stays and the report names it.
	base, err := req.Owned.Base()
	if err != nil {
		return repair
	}
	var edits []edit.Edit
	seen := set.New[int]()
	for _, r := range runs {
		for _, h := range forkscope.Grown(base, repair.Text, r.first, r.last) {
			if !seen.Contains(h.J1) {
				seen.Add(h.J1)
				edits = append(edits, foldHunk(repair.Text, h))
			}
		}
	}
	if len(edits) == 0 {
		return repair
	}
	at := req
	at.Content, at.Scope, at.Owned = repair.Text, repair.Scope, nil
	f := openFile(at, kindOf(req.Path, repair.Text))
	res := f.ApplyComments(edits)
	repair.refuse(res.Refused)
	if len(res.Applied) == 0 {
		return repair
	}
	again := at
	again.Content, again.Scope = f.Text(), edit.Nowhere()
	landed := Fix(again)
	repair.Findings, repair.Kept = landed.Findings, landed.Kept
	repair.Text, repair.Changed, repair.Scope = f.Text(), f.Text() != req.Content, f.Scope()
	return repair
}

// foldHunk answers the edit that joins the lines h added onto the last line it
// keeps, with the comment marker of each joined line dropped.
func foldHunk(text string, h forkscope.Hunk) edit.Edit {
	rows := strings.Split(text, "\n")[h.J1:h.J2]
	keep := max(h.Had, 1)
	lines := slices.Clone(rows[:keep])
	for _, row := range rows[keep:] {
		if prose := commentProse(row); prose != "" {
			lines[keep-1] = strings.TrimRight(lines[keep-1], " \t") + " " + prose
		}
	}
	return edit.Rows(text, h.J1, h.J2-1, 0, lines)
}

// commentProse answers what a comment line says, without its indent and marker.
func commentProse(row string) string {
	text := strings.TrimSpace(row)
	for _, marker := range []string{"//", "#", "*"} {
		if rest, ok := strings.CutPrefix(text, marker); ok {
			return strings.TrimSpace(rest)
		}
	}
	return text
}

// upstreamRuns drops each block finding on a run of text the fork did not make
// longer. The base already had that run at that length. It is the base's
// finding, and the fork's edit inside it stays as the fork wrote it.
func upstreamRuns(owned *forkscope.Scope, text string, repair Repair) Repair {
	if owned == nil || owned.All() {
		return repair
	}
	base, err := owned.Base()
	if err != nil {
		return repair
	}
	grew := func(first, last int) bool { return len(forkscope.Grown(base, text, first, last)) > 0 }
	findings := repair.Findings[:0:0]
	for _, f := range repair.Findings {
		if !blockRules.Contains(f.ID) || grew(f.Line, max(f.Line, f.EndLine)) {
			findings = append(findings, f)
		}
	}
	kept := repair.Kept[:0:0]
	for _, h := range repair.Kept {
		if !blockRules.Contains(h.ID) || h.LineNo < 1 || grew(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			kept = append(kept, h)
		}
	}
	repair.Findings, repair.Kept = findings, kept
	return repair
}

// landedRemovals keeps each removal that text, the repair as it lands, made.
func landedRemovals(before, text string, removed []string) []string {
	cut := forkscope.Removed(before, text)
	var out []string
	for _, r := range removed {
		if strings.Contains(cut, strings.TrimSpace(r)) {
			out = append(out, r)
		}
	}
	return out
}

func ownedFindings(findings []ste.Finding, owned *forkscope.Scope) []ste.Finding {
	var out []ste.Finding
	for _, f := range findings {
		if owned.Holds(f.Line, f.EndLine) {
			out = append(out, f)
		}
	}
	return out
}

func ownedHits(hits []tombstones.Hit, owned *forkscope.Scope) []tombstones.Hit {
	var out []tombstones.Hit
	for _, h := range hits {
		if owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			out = append(out, h)
		}
	}
	return out
}

// Within keeps what a tree run from root found on lines the fork wrote. A
// repository rule names its file relative to root. Every fixture expectation
// stays, because a fixture is the repository's own test.
func (t TreeRepair) Within(own *forkscope.Lines, root string) TreeRepair {
	if own == nil {
		return t
	}
	findings := t.Findings[:0:0]
	for _, f := range t.Findings {
		path := f.Path
		if RepoIDs.Contains(f.ID) && !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if own.Holds(path, f.Line, f.EndLine) {
			findings = append(findings, f)
		}
	}
	kept := t.Kept[:0:0]
	for _, k := range t.Kept {
		if own.Holds(k.Path, k.LineNo, max(k.LineNo, k.EndLineNo)) {
			kept = append(kept, k)
		}
	}
	t.Findings, t.Kept = findings, kept
	return t
}
