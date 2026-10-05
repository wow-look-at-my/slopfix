// handfix.go writes the repair of `slopfix fix` over a line a hand edit changes.
//
// The repair of the file as it stands on disk is F. A line F changes carries
// an auto-fixable finding, and only `slopfix fix` may write it. An Edit or
// MultiEdit that changes such a line goes through as `slopfix fix` writes it:
// the edit is replayed. The result is repaired, and the repair is kept on
// every line the edit wrote. The edit then replaces those lines whole. A
// Write is repaired whole by the write guard. A line the edit only inserts
// beside is not a line it changes.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
)

// handFixNote tells the writer which line the repair wrote in place of the edit.
const handFixNote = "%s:%d: [%s] the edit changed a line `slopfix fix` repairs, so the line is as slopfix fix writes it"

// replacement is an edit as the tool applies it, with replace_all honored.
type replacement struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

// toolEdits carries the edit fields of Edit and MultiEdit together.
type toolEdits struct {
	replacement
	Edits []replacement `json:"edits"`
}

// replacementsOf reads the edits the named tool carries, in the order it applies them.
func replacementsOf(tool string, in toolEdits) []replacement {
	if tool == "Edit" {
		return []replacement{in.replacement}
	}
	return in.Edits
}

// replay applies the edits the way the tool does. It answers false when the
// tool itself refuses them, because an old_string is absent or ambiguous.
func replay(src string, edits []replacement) (string, bool) {
	out := src
	for _, e := range edits {
		n := strings.Count(out, e.OldString)
		if e.OldString == "" || n == 0 || (n > 1 && !e.ReplaceAll) {
			return "", false
		}
		out = strings.Replace(out, e.OldString, e.NewString, -1)
	}
	return out, true
}

// changedLines answers the 1-based lines of before that after replaces or
// deletes.
func changedLines(before, after string) []int {
	a := strings.SplitAfter(before, "\n")
	b := strings.SplitAfter(after, "\n")
	var out []int
	for _, op := range difflib.NewMatcherWithJunk(a, b, false, nil).GetOpCodes() {
		if op.Tag != 'r' && op.Tag != 'd' {
			continue
		}
		for i := op.I1; i < op.I2; i++ {
			out = append(out, i+1)
		}
	}
	return out
}

// fixOf answers what `slopfix fix` makes of src. A panic in a fixer is an
// error, so a failed computation is never read as a clean file.
func fixOf(req slopfix.Request) (repair slopfix.Repair, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("slopfix fix panicked on %s: %v", req.Path, r)
		}
	}()
	return slopfix.Fix(req), nil
}

// handRepair is an edit rewritten so the lines it changes are as `slopfix
// fix` writes them.
type handRepair struct {
	old   string
	new   string
	notes []string
}

// apply puts the rewritten edit into the payload as the tool reads it. A
// MultiEdit becomes a single edit, since every edit it carried is in it.
func (h *handRepair) apply(tool string, in *writeInput, raw map[string]any) {
	delete(raw, "replace_all")
	if tool == "Edit" {
		in.OldString, in.NewString = h.old, h.new
		raw["old_string"], raw["new_string"] = h.old, h.new
		return
	}
	in.Edits = []replacement{{OldString: h.old, NewString: h.new}}
	raw["edits"] = []any{map[string]any{"old_string": h.old, "new_string": h.new}}
}

// cannotFix is the refusal for a repair that could not be computed.
func cannotFix(path string, err error) []string {
	return []string{fmt.Sprintf("slopfix cannot compute what `slopfix fix %s` changes: %v", path, err)}
}

// cannotScope is the refusal for a fork whose lines could not be read.
func cannotScope(path string, err error) []string {
	return []string{fmt.Sprintf("slopfix cannot tell which lines of %s this fork wrote: %v", path, err)}
}

// handFix answers the rewrite of an edit that changes a line F changes, or nil
// when the edit may go on to the repair as it stands. It answers a refusal
// only when it cannot compute the rewrite. A path with no file yet is a new
// file, and only the repair judges it. In a fork, F changes only the lines the
// fork wrote, so a file the fork never touched is left to the repair.
func handFix(tool string, toolInput []byte, path string, rules []slopfix.Rule, ids []string, owned forkLines) (*handRepair, []string) {
	if path == "" || (tool != "Edit" && tool != "MultiEdit") {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, cannotFix(path, err)
	}
	before := string(data)
	in, err := decodeEdits(toolInput)
	if err != nil {
		return nil, []string{fmt.Sprintf("slopfix cannot read the edits to %s: %v", path, err)}
	}
	after, ok := replay(before, replacementsOf(tool, in))
	if !ok {
		return nil, nil // the tool refuses an edit it cannot place
	}
	scope, err := owned(before)
	if err != nil {
		return nil, cannotScope(path, err)
	}
	if scope != nil && scope.Empty() {
		return nil, nil
	}
	req := slopfix.Request{Content: before, Path: path, Rules: rules, IDs: ids, MaxCommentLines: hookMaxLines, Owned: scope}
	fixed, err := fixOf(req)
	if err != nil {
		return nil, cannotFix(path, err)
	}
	fixable := set.Of(changedLines(before, fixed.Text)...)
	var touched []int
	for _, line := range changedLines(before, after) {
		if fixable.Contains(line) {
			touched = append(touched, line)
		}
	}
	if len(touched) == 0 {
		return nil, nil
	}
	return rewriteHand(req, fixed.Text, after, touched, owned)
}

// fixAfter answers what `slopfix fix` makes of the file the edit leaves.
func fixAfter(req slopfix.Request, after string, owned forkLines) (string, []string) {
	scope, err := owned(after)
	if err != nil {
		return "", cannotScope(req.Path, err)
	}
	whole := req
	whole.Content, whole.Owned = after, scope
	repaired, err := fixOf(whole)
	if err != nil {
		return "", cannotFix(req.Path, err)
	}
	return repaired.Text, nil
}

// rewriteHand repairs the file the edit leaves, keeps that repair on every
// line the edit wrote, and answers the edit that writes the result. A line
// the hand repaired, so that fix no longer changes it, is put back as fix
// writes it.
func rewriteHand(req slopfix.Request, fixedBefore, after string, touched []int, owned forkLines) (*handRepair, []string) {
	before, path := req.Content, req.Path
	repaired, refused := fixAfter(req, after, owned)
	if refused != nil {
		return nil, refused
	}
	landed := restoreRepairs(before, fixedBefore, after, set.Of(changedLines(after, repaired)...))
	if landed != after {
		if repaired, refused = fixAfter(req, landed, owned); refused != nil {
			return nil, refused
		}
	}
	result := keepWritten(landed, repaired, writtenLines(before, landed))
	if result == after {
		return nil, nil
	}
	if result == before {
		return nil, []string{fmt.Sprintf("the edit to %s changes only lines `slopfix fix` writes back as they stand, so it leaves the file unchanged", path)}
	}
	names, err := ruleIDsOn(req, touched)
	if err != nil {
		return nil, cannotFix(path, err)
	}
	span, text := lineSpan(before, result)
	h := &handRepair{old: span, new: text}
	for _, line := range touched {
		h.notes = append(h.notes, fmt.Sprintf(handFixNote, path, line, strings.Join(names[line], ", ")))
	}
	return h, nil
}

// restoreRepairs answers after with each line put back as fix writes it. This
// happens where the edit changed a line fix repairs and fix leaves the edited
// line alone. This is because the edit performed the repair by hand.
// stillFixed holds the 1-based lines of after that fix changes. Only a line
// each diff pairs one to one is put back.
func restoreRepairs(before, fixedBefore, after string, stillFixed set.Set[int]) string {
	edited := pairs(before, after)
	repaired := pairs(before, fixedBefore)
	fixedLines := strings.SplitAfter(fixedBefore, "\n")
	out := strings.SplitAfter(after, "\n")
	for i, j := range edited {
		k, ok := repaired[i]
		if !ok || stillFixed.Contains(j+1) {
			continue
		}
		out[j] = fixedLines[k]
	}
	return strings.Join(out, "")
}

// pairs maps each 0-based line of x that y replaces one for one to the line
// of y that replaces it.
func pairs(x, y string) map[int]int {
	a := strings.SplitAfter(x, "\n")
	b := strings.SplitAfter(y, "\n")
	out := map[int]int{}
	for _, op := range difflib.NewMatcherWithJunk(a, b, false, nil).GetOpCodes() {
		if op.Tag != 'r' || op.I2-op.I1 != op.J2-op.J1 {
			continue
		}
		for k := 0; k < op.I2-op.I1; k++ {
			out[op.I1+k] = op.J1 + k
		}
	}
	return out
}

// writtenLines answers the 0-based lines of after that the edit wrote.
func writtenLines(before, after string) set.Set[int] {
	a := strings.SplitAfter(before, "\n")
	b := strings.SplitAfter(after, "\n")
	out := set.New[int]()
	for _, op := range difflib.NewMatcherWithJunk(a, b, false, nil).GetOpCodes() {
		if op.Tag == 'e' {
			continue
		}
		for j := op.J1; j < op.J2; j++ {
			out.Add(j)
		}
	}
	return out
}

// keepWritten takes from fixed every change that lands on lines of after the
// edit wrote, and leaves after as it stands everywhere else. A change that
// also rewrites a line the edit never wrote, such as a paragraph join, is
// left out, so no word the write did not touch is changed.
func keepWritten(after, fixed string, written set.Set[int]) string {
	a := strings.SplitAfter(after, "\n")
	b := strings.SplitAfter(fixed, "\n")
	var out strings.Builder
	for _, op := range difflib.NewMatcherWithJunk(a, b, false, nil).GetOpCodes() {
		switch {
		case op.Tag == 'e':
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
		case op.I2-op.I1 == op.J2-op.J1:
			for k := 0; k < op.I2-op.I1; k++ {
				line := a[op.I1+k]
				if written.Contains(op.I1 + k) {
					line = b[op.J1+k]
				}
				out.WriteString(line)
			}
		case within(written, op.I1, op.I2):
			out.WriteString(strings.Join(b[op.J1:op.J2], ""))
		default:
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
		}
	}
	return out.String()
}

// within reports whether every line from i1 up to i2 is a written line. A
// change that only inserts is within when a line beside it is written.
func within(written set.Set[int], i1, i2 int) bool {
	if i1 == i2 {
		return written.Contains(i1-1) || written.Contains(i1)
	}
	for i := i1; i < i2; i++ {
		if !written.Contains(i) {
			return false
		}
	}
	return true
}

// lineSpan answers the whole lines of before that result replaces, and what
// replaces them. The span grows a line each way until it appears in before a
// single time, so the edit tool can place it.
func lineSpan(before, result string) (span, text string) {
	a := strings.SplitAfter(before, "\n")
	b := strings.SplitAfter(result, "\n")
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	for {
		span = strings.Join(a[p:len(a)-s], "")
		text = strings.Join(b[p:len(b)-s], "")
		if (span != "" && strings.Count(before, span) == 1) || (p == 0 && s == 0) {
			return span, text
		}
		p = max(p-1, 0)
		s = max(s-1, 0)
	}
}

// decodeEdits reads the Edit and MultiEdit fields of a tool input.
func decodeEdits(toolInput []byte) (toolEdits, error) {
	var in toolEdits
	err := json.Unmarshal(toolInput, &in)
	return in, err
}

// ruleIDsOn names the rules behind each line. A finding the report places on
// the line names its rule. A line no finding names is put to each rule
// category alone, and the categories whose repair changes it are named.
func ruleIDsOn(req slopfix.Request, lines []int) (map[int][]string, error) {
	names := map[int][]string{}
	add := func(line int, id string) {
		if !slices.Contains(names[line], id) {
			names[line] = append(names[line], id)
		}
	}
	wanted := set.Of(lines...)
	report := slopfix.Report(req)
	for _, f := range report.Findings {
		for line := f.Line; line <= max(f.Line, f.EndLine); line++ {
			if wanted.Contains(line) {
				add(line, f.ID)
			}
		}
	}
	for _, hit := range report.Kept {
		if wanted.Contains(hit.LineNo) {
			add(hit.LineNo, hit.ID)
		}
	}
	var unnamed []int
	for _, line := range lines {
		if len(names[line]) == 0 {
			unnamed = append(unnamed, line)
		}
	}
	if len(unnamed) == 0 {
		return names, nil
	}
	categories := req.Rules
	if len(categories) == 0 {
		categories = slopfix.AllRules
	}
	for _, rule := range categories {
		alone := req
		alone.Rules = []slopfix.Rule{rule}
		fixed, err := fixOf(alone)
		if err != nil {
			return nil, err
		}
		changed := set.Of(changedLines(req.Content, fixed.Text)...)
		for _, line := range unnamed {
			if changed.Contains(line) {
				add(line, string(rule))
			}
		}
	}
	for _, line := range unnamed {
		if len(names[line]) == 0 {
			add(line, "several rules together")
		}
	}
	return names, nil
}
