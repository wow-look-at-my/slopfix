package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// repo/binary: a compiled binary committed to the tree. The repair removes it,
// because the build makes it and the history keeps it.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDBinary,
		Category: RuleRepo,
		Detect:   detectBinary,
		Autofix:  autofixBinary,
		Cases: []RuleCase{{Name: IDBinary, Files: map[string]string{
			"bin/tool": "\x7fELF\x02\x01\x01\x00rest of the executable",
		}}},
	})
}

// detectBinary answers every committed binary.
func detectBinary(c RuleCase) []ste.Finding { return treeFindings(c, IDBinary) }

// autofixBinary removes the binary from the tree.
func autofixBinary(c RuleCase) RuleCase { return treeAutofix(c, IDBinary) }
