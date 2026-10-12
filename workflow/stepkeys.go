package workflow

import (
	"regexp"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

var (
	// stepItem matches the line that opens a step: a sequence dash and its first key.
	stepItem = regexp.MustCompile(`^(\s*)-\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*:`)
	// stepKey captures a key line's indentation, so a caller can tell the step's own key from a nested one.
	stepKey = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_.-]*)\s*:`)
)

// duplicateStepKeys reports a key a step names twice.
//
// GitHub rejects the whole file for one, before it makes any job, so the run
// that would have reported it never starts. The error names the file and not
// the step, and every other workflow in the repository goes with it.
//
// Only a key at the step's own indentation counts. A key one level in belongs
// to with:, env: or another mapping, where the same name is free to appear
// again.
func duplicateStepKeys(content string) []ste.Finding {
	rows := lines(content)
	var out []ste.Finding
	indent, seen := -1, set.New[string]()
	for i, row := range rows {
		if item := stepItem.FindStringSubmatch(row); item != nil {
			indent = len(item[1]) + 2
			seen = set.Of(item[2])
			continue
		}
		key := stepKey.FindStringSubmatch(row)
		if indent < 0 || key == nil || len(key[1]) != indent {
			continue
		}
		if seen.Contains(key[2]) {
			out = append(out, ste.Finding{
				Line:   i + 1,
				ID:     IDDuplicateStepKey,
				Rule:   "this step names a key it already has",
				Detail: key[2],
				Fix:    "GitHub rejects the whole file for a repeated step key, before it makes any job. Keep the value the step needs and delete the other line.",
			})
		}
		seen.Add(key[2])
	}
	return out
}
