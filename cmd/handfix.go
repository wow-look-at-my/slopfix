// handfix.go refuses a write that changes a line `slopfix fix` would repair.
//
// The repair of the file as it stands on disk is F. A line F changes carries
// an auto-fixable finding, and only `slopfix fix` may change it. An edit that
// changes such a line is denied, and names the lines and the rules. A line
// the edit only inserts beside is not a line it changes.
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

// handFixAdvice is what a refusal tells the writer to do instead.
const handFixAdvice = "run `slopfix fix %s`, then make your change; hand-edits to lines slopfix auto-fixes are refused"

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

// handFix answers the refusal for a write that changes a line F changes, or
// nil when the write may go on to the repair. A path with no file yet is a new
// file, and only the repair judges it. In a fork, F changes only the lines the
// fork wrote, so a file the fork never touched refuses nothing.
func handFix(tool string, toolInput []byte, path string, content string, rules []slopfix.Rule, ids []string, owned forkLines) []string {
	if path == "" || (tool != "Write" && tool != "Edit" && tool != "MultiEdit") {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("slopfix cannot compute what `slopfix fix %s` changes: %v", path, err)}
	}
	before := string(data)
	after := content
	if tool != "Write" {
		in, err := decodeEdits(toolInput)
		if err != nil {
			return []string{fmt.Sprintf("slopfix cannot read the edits to %s: %v", path, err)}
		}
		var ok bool
		// The tool refuses an edit it cannot place, so nothing reaches the file.
		if after, ok = replay(before, replacementsOf(tool, in)); !ok {
			return nil
		}
	}
	scope, err := owned(before)
	if err != nil {
		return []string{fmt.Sprintf("slopfix cannot tell which lines of %s this fork wrote: %v", path, err)}
	}
	if scope != nil && scope.Empty() {
		return nil
	}
	req := slopfix.Request{Content: before, Path: path, Rules: rules, IDs: ids, MaxCommentLines: hookMaxLines, Owned: scope}
	fixed, err := fixOf(req)
	if err != nil {
		return []string{fmt.Sprintf("slopfix cannot compute what `slopfix fix %s` changes: %v", path, err)}
	}
	fixable := set.Of(changedLines(before, fixed.Text)...)
	var touched []int
	for _, line := range changedLines(before, after) {
		if fixable.Contains(line) {
			touched = append(touched, line)
		}
	}
	if len(touched) == 0 {
		return nil
	}
	names, err := ruleIDsOn(req, touched)
	if err != nil {
		return []string{fmt.Sprintf("slopfix cannot compute what `slopfix fix %s` changes: %v", path, err)}
	}
	out := []string{fmt.Sprintf(handFixAdvice, path)}
	for _, line := range touched {
		out = append(out, fmt.Sprintf("%s:%d: [%s]", path, line, strings.Join(names[line], ", ")))
	}
	return out
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
