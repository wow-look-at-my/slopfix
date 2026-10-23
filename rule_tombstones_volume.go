// tombstones/comment-volume: a comment run longer than the cap.

package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// tombstones/comment-volume: a comment run longer than the cap. The repair cuts
// the run from its end down to the cap, the way comments/length cuts.
func init() {
	RegisterRule(RuleSpec{
		ID:       tombstones.IDVolume,
		Category: RuleTombstones,
		Detect:   detectVolume,
		Autofix:  autofixVolume,
		Cases: []RuleCase{
			{Name: tombstones.IDVolume, Path: "main.go", Text: volumeCase()},
			{Name: "a run with no sentence end goes whole", Path: "main.go", Text: fixtureHeader + "package main\n\n" + alignedRun() + volumeCode(), Want: fixtureHeader + "package main\n\n" + volumeCode()},
		},
	})
}

// detectVolume answers every overlong comment run this rule reports.
func detectVolume(c RuleCase) []ste.Finding {
	return caseFindings(c, tombstones.IDVolume)
}

// autofixVolume cuts the run from its end down to the cap.
func autofixVolume(c RuleCase) RuleCase {
	return caseAutofix(c, tombstones.IDVolume)
}

// volumeCase is a comment run longer than the default cap, above code longer
// than the comment. The run outweighs no code and only the cap reads it.
func volumeCase() string {
	out := fixtureHeader + "package main\n\n"
	for range tombstones.DefaultMaxCommentLines + 2 {
		out += "// The loop reads each value from the input and adds it to the running total.\n"
	}
	return out + volumeCode()
}

// volumeCode is a function longer than any comment run the cases put on it.
func volumeCode() string {
	out := "func main() {\n"
	for range tombstones.DefaultMaxCommentLines * 3 {
		out += "\ttotal += readValue(input, limit, offset)\n"
	}
	return out + "}\n"
}

// alignedRun is a run over the cap whose lines are aligned columns. No line
// ends a sentence, so no cut lands on a sentence end.
func alignedRun() string {
	out := ""
	for _, name := range []string{"input", "header", "body", "cache", "index", "table", "queue", "limit",
		"total", "buffer", "socket", "stream", "record", "schema", "token", "value"} {
		out += "//   " + name + "    the loop reads it\n"
	}
	return out
}
