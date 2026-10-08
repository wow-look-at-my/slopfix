package slopfix

import (
	"strconv"

	"github.com/wow-look-at-my/slopfix/ste"
)

// repo/near-duplicate: files with one name that hold almost the same lines. The
// repair deletes the duplicated functions or classes from the later copy and
// keeps whatever that copy alone holds, so the file itself stays.
func init() {
	head := "package tool\n\n"
	unique := "// Only answers the value nothing else answers.\n" +
		"func Only() int {\n" +
		"\treturn 2\n" +
		"}\n"
	RegisterRule(RuleSpec{
		ID:       IDNearDuplicate,
		Category: RuleRepo,
		Detect:   detectNearDuplicate,
		Autofix:  autofixNearDuplicate,
		Cases: []RuleCase{{Name: IDNearDuplicate, Files: map[string]string{
			"a/tool.go": head + sharedFunctions(),
			"b/tool.go": head + sharedFunctions() + unique,
		}}},
	})
}

// detectNearDuplicate answers every copy the rule reports under root.
func detectNearDuplicate(c RuleCase) []ste.Finding { return treeFindings(c, IDNearDuplicate) }

// autofixNearDuplicate cuts the duplicated portions out of the later copy.
func autofixNearDuplicate(c RuleCase) RuleCase { return treeAutofix(c, IDNearDuplicate) }

// sharedFunctions writes the function run both copies hold. The later copy adds
// one function of its own, which is what keeps the pair above the duplicate
// share. And what the repair leaves in place.
func sharedFunctions() string {
	out := ""
	for n := range 8 {
		name := "Reads" + strconv.Itoa(n)
		out += "// " + name + " reads the value the caller asked for.\n" +
			"func " + name + "() int {\n" +
			"\tvalue := 1\n" +
			"\treturn value\n" +
			"}\n\n"
	}
	return out
}
