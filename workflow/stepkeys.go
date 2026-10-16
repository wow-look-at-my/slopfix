package workflow

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
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
				Fix:    "GitHub rejects the whole file for a repeated step key, before it makes any job. Delete the pair the step does not need. `slopfix fix` keeps the last one.",
			})
		}
		seen.Add(key[2])
	}
	return out
}

// stepKeyStart is a key a step names, and the row it starts on.
type stepKeyStart struct {
	name string
	row  int
}

// stepKeyBlocks answers every step of the file as the rows its keys start on,
// and the row each step ends before.
func stepKeyBlocks(rows []string) (starts [][]stepKeyStart, ends []int) {
	for i, row := range rows {
		item := stepItem.FindStringSubmatch(row)
		if item == nil {
			continue
		}
		indent := len(item[1]) + 2
		keys := []stepKeyStart{{item[2], i}}
		end := len(rows)
		for j := i + 1; j < len(rows); j++ {
			trimmed := strings.TrimLeft(rows[j], " ")
			if trimmed != "" && len(rows[j])-len(trimmed) < indent {
				end = j
				break
			}
			if key := stepKey.FindStringSubmatch(rows[j]); key != nil && len(key[1]) == indent {
				keys = append(keys, stepKeyStart{key[2], j})
			}
		}
		starts, ends = append(starts, keys), append(ends, end)
	}
	return starts, ends
}

// dropDuplicateKeys keeps the last value of a key a step repeats. This is
// the value a reader that takes each pair in turn ends with, and deletes the
// earlier pairs with the rows their values take.
func dropDuplicateKeys(content string) []edit.Edit {
	rows := lines(content)
	starts, ends := stepKeyBlocks(rows)
	drop := set.New[int]()
	for n, keys := range starts {
		for k, key := range keys {
			end := ends[n]
			if k+1 < len(keys) {
				end = keys[k+1].row
			}
			if !laterKey(keys[k+1:], key.name) {
				continue
			}
			for row := key.row; row < end; row++ {
				drop.Add(row)
			}
		}
	}
	var out []edit.Edit
	for row := 0; row < len(rows); row++ {
		if !drop.Contains(row) {
			continue
		}
		last := row
		for drop.Contains(last + 1) {
			last++
		}
		item := stepItem.FindStringSubmatch(rows[row])
		if item == nil || last+1 >= len(rows) {
			out = append(out, edit.Lines(content, row, last, 0, nil))
			row = last
			continue
		}
		// The step's first key sits on its dash row, so the next key takes the dash.
		merged := item[1] + "- " + strings.TrimLeft(rows[last+1], " ")
		out = append(out, rewrite(content, row, last+1, []string{merged}))
		row = last + 1
	}
	return out
}

// laterKey reports whether a key named name starts among keys.
func laterKey(keys []stepKeyStart, name string) bool {
	for _, key := range keys {
		if key.name == name {
			return true
		}
	}
	return false
}
