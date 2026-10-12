package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/ste"
)

// counts/inventory-list: prose that lists the members of a set the code
// defines. The repair deletes the enumeration.
func init() {
	RegisterRule(RuleSpec{
		ID:       counts.IDList,
		Category: RuleCounts,
		Detect:   detectInventoryList,
		Autofix:  autofixInventoryList,
		Cases: []RuleCase{
			{Name: "commands", Path: "x.md", Text: "The commands are `check`, `hook`, `lsp`, `ratchet`, `version`, `completion` and `help`.\n"},
			{Name: "frame", Path: "x.md", Text: "# Tool\n\nThe tool reads prose.\n\nThe modes are `fast` and `slow`.\n"},
			{Name: "list-block", Path: "x.md", Text: "# Tool\n\nThe tool reads prose.\n\nRun one of these:\n\n- `check` reads a file.\n- `fix` repairs a file.\n- `hook` answers a hook.\n"},
			{Name: "table", Path: "x.md", Text: "# Tool\n\nThe tool reads prose.\n\n| Name | Use |\n|---|---|\n| `a` | reads |\n| `b` | writes |\n| `c` | checks |\n"},
			{Name: "catalog-fence", Path: "x.md", Text: "# Tool\n\nThe tool reads prose.\n\n```sh\nslopfix check .\nslopfix fix .\nslopfix hook\n```\n"},
			{Name: "split", Path: "x.md", Text: "# Tool\n\nThe tool reads prose.\n\nThe tool reads `a`. The tool reads `b`. The tool reads `c`.\n"},
			{Name: "comment", Path: "x.go", Text: "package x\n\n// The modes are `fast`, `slow` and `safe`.\nconst mode = 1\n"},
			{Name: "message", Text: "I changed `a.go`, `b.go` and `c.go`.\n"},
		},
	})
}

// detectInventoryList answers each enumeration the case carries.
func detectInventoryList(c RuleCase) []ste.Finding {
	return caseFindings(c, counts.IDList)
}

// autofixInventoryList deletes each enumeration the case carries.
func autofixInventoryList(c RuleCase) RuleCase {
	return caseAutofix(c, counts.IDList)
}

// listRule and listFix are what a finding of this rule says.
const (
	listRule = "a list of what the code holds goes stale when the code changes"
	listFix  = "Name the set and let the reader ask the code. `slopfix fix` deletes the list."
)

// listFindings answers each enumeration in one document.
func listFindings(doc string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range counts.FindLists(doc) {
		out = append(out, ste.Finding{Line: hit.LineNo, ID: counts.IDList, Rule: listRule, Detail: hit.Detail, Fix: listFix})
	}
	return out
}

// commentListFindings answers each enumeration in the comments of a source file or a workflow.
func commentListFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range commentfix.CheckLists(path, content) {
		out = append(out, ste.Finding{Line: hit.Line, ID: counts.IDList, Rule: listRule, Detail: hit.Sentence, Fix: listFix})
	}
	return out
}
