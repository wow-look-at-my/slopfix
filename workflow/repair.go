// repair.go rewrites what a workflow rule finds, wherever a rewrite says the
// same thing the author meant.
//
// A newline is syntax in YAML, so every repair here works on whole lines and
// never reflows. What no rewrite can say is left to the author, and the report
// keeps naming it.
package workflow

import (
	"regexp"
	"strings"
)

// Repair is a workflow as this binary would write it.
type Repair struct {
	// Text is the repaired content. It equals the input when Changed is false.
	Text string
	// Changed reports whether any rewrite applied.
	Changed bool
	// Removed names each line the repair cut.
	Removed []string
}

// Fix applies every repair the caller keeps. keeps takes a rule ID, so a run
// that named a rule gets that rule alone.
func Fix(content string, keeps func(string) bool) Repair {
	text := content
	var removed []string

	if keeps(IDNeuteredGate) {
		var cut []string
		text, cut = ungate(text)
		removed = append(removed, cut...)
	}
	if keeps(IDCommentBlock) {
		var cut []string
		text, cut = joinCommentBlocks(text)
		removed = append(removed, cut...)
	}
	if keeps(IDAllBuildsJob) {
		text = renameGuardedJob(text)
	}
	return Repair{Text: text, Changed: text != content, Removed: removed}
}

// ungate deletes the continue-on-error a gate step hides behind. The step stays
// and starts failing, which is what a gate is for.
func ungate(content string) (string, []string) {
	findings := neuteredGates(content)
	if len(findings) == 0 {
		return content, nil
	}
	rows := lines(content)
	drop := make(map[int]bool)
	for _, f := range findings {
		open := stepOpen.FindStringSubmatch(rows[f.Line-1])
		indent := len(open[1])
		for j := f.Line - 1; j < len(rows); j++ {
			if next := listItem.FindStringSubmatch(rows[j]); j > f.Line-1 && next != nil && len(next[1]) <= indent {
				break
			}
			if allowedToFail.MatchString(rows[j]) {
				drop[j] = true
			}
		}
	}
	return without(rows, drop, content)
}

// joinCommentBlocks folds a run of comment lines into the single line the rule
// allows, keeping every word.
func joinCommentBlocks(content string) (string, []string) {
	findings := commentBlocks(content)
	if len(findings) == 0 {
		return content, nil
	}
	rows := lines(content)
	drop := make(map[int]bool)
	for _, f := range findings {
		var said []string
		for j := f.Line - 1; j < f.EndLine && j < len(rows); j++ {
			trimmed := strings.TrimSpace(rows[j])
			if !strings.HasPrefix(trimmed, "#") {
				continue
			}
			if rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "#")); rest != "" {
				said = append(said, rest)
			}
			if j > f.Line-1 {
				drop[j] = true
			}
		}
		indent := rows[f.Line-1][:len(rows[f.Line-1])-len(strings.TrimLeft(rows[f.Line-1], " \t"))]
		rows[f.Line-1] = strings.TrimRight(indent+"# "+strings.Join(said, " "), " ")
	}
	return without(rows, drop, content)
}

// guardedKey matches the job key the org's gate reserves.
var guardedKey = regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(GuardedName) + `\s*:`)

// renameGuardedJob renames a job that shadows the required status, and the
// needs entries that point at it. The name is the whole finding, so renaming it
// is the whole repair.
func renameGuardedJob(content string) string {
	rows := lines(content)
	renamed := false
	for i, row := range rows {
		if guardedKey.MatchString(row) {
			rows[i] = strings.Replace(row, GuardedName, replacementName, 1)
			renamed = true
		}
	}
	if !renamed {
		return content
	}
	for i, row := range rows {
		trimmed := strings.TrimSpace(row)
		if strings.HasPrefix(trimmed, "needs:") || strings.HasPrefix(trimmed, "- ") {
			rows[i] = strings.ReplaceAll(row, GuardedName, replacementName)
		}
	}
	return strings.Join(rows, "\n") + tail(content)
}

// replacementName is what a shadowing job is renamed to: the same job, under a
// name the gate does not reserve.
const replacementName = "builds"

// without drops the marked lines and reports what went.
func without(rows []string, drop map[int]bool, content string) (string, []string) {
	var kept, removed []string
	for i, row := range rows {
		if drop[i] {
			removed = append(removed, strings.TrimSpace(row))
			continue
		}
		kept = append(kept, row)
	}
	return strings.Join(kept, "\n") + tail(content), removed
}

// tail keeps the trailing newline the split dropped, so a repair does not
// rewrite the last byte of every file it touches.
func tail(content string) string {
	if strings.HasSuffix(content, "\n") {
		return "\n"
	}
	return ""
}
