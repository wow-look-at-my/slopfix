package slopfix

import (
	"strconv"

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
		Cases:    []RuleCase{{Name: tombstones.IDVolume, Path: "main.go", Text: volumeCase()}},
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

// volumeCase is a comment run longer than the default cap.
func volumeCase() string {
	out := "package main\n\n"
	for i := range 16 {
		out += "// The loop reads value " + strconv.Itoa(i) + " from the input and adds it to the running total.\n"
	}
	return out + "func main() {}\n"
}
