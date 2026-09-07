package workflow

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// GateMarkers name the steps this rule protects, a wrapper step included.
var GateMarkers = []string{"slopfix", "ste-lint", "common-checks"}

var (
	// stepOpen matches the line that opens a step in a steps list.
	stepOpen = regexp.MustCompile(`^(\s*)-\s+(?:uses|name)\s*:`)
	// listItem matches any step line, for finding where this step ends.
	listItem = regexp.MustCompile(`^(\s*)-\s`)
	// usesLine matches the key that names what a step runs.
	usesLine = regexp.MustCompile(`^\s*-?\s*uses\s*:`)
	// allowedToFail matches the key that turns a red step green.
	allowedToFail = regexp.MustCompile(`^\s*continue-on-error\s*:\s*true\b`)
)

// neuteredGates reports each step that runs a gate under continue-on-error.
//
// A rule is only as strong as the step that runs it. A step allowed to fail is
// not a gate, and a gate that says nothing about being switched off is
// decoration. Nothing in the rule's own output shows this, which is what makes
// it worth reaching for.
//
// The step's own indentation bounds it. A YAML walk answers the same question,
// and this rule must report the LINE the step opens on, which is what a reader
// puts a cursor on.
func neuteredGates(content string) []ste.Finding {
	rows := lines(content)
	var out []ste.Finding
	for i := range rows {
		open := stepOpen.FindStringSubmatch(rows[i])
		if open == nil {
			continue
		}
		indent := len(open[1])
		runs, neutered := "", false
		for j := i; j < len(rows); j++ {
			if next := listItem.FindStringSubmatch(rows[j]); j > i && next != nil && len(next[1]) <= indent {
				break
			}
			if usesLine.MatchString(rows[j]) && namesAGate(rows[j]) {
				runs = strings.TrimSpace(rows[j])
			}
			if allowedToFail.MatchString(rows[j]) {
				neutered = true
			}
		}
		if runs == "" || !neutered {
			continue
		}
		out = append(out, ste.Finding{
			Line:   i + 1,
			ID:     IDNeuteredGate,
			Rule:   "this step runs a gate under continue-on-error",
			Detail: runs,
			Fix:    "A step allowed to fail is not a gate. Remove continue-on-error, or remove the step and say so out loud.",
		})
	}
	return out
}

func namesAGate(line string) bool {
	for _, marker := range GateMarkers {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}
