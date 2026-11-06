// rule_counts_inventorylist.go holds the rule counts inventorylist code.

package slopfix

import _ "embed"

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
			{Name: "frame", Path: "x.md", Text: embeddedMd},
			{Name: "list-block", Path: "x.md", Text: embeddedMd2},
			{Name: "table", Path: "x.md", Text: embeddedMd3},
			{Name: "catalog-fence", Path: "x.md", Text: embeddedMd4},
			{Name: "split", Path: "x.md", Text: embeddedMd5},
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

//go:embed testdata/embedded.md
var embeddedMd string

//go:embed testdata/embedded-2.md
var embeddedMd2 string

//go:embed testdata/embedded-3.md
var embeddedMd3 string

//go:embed testdata/embedded-4.md
var embeddedMd4 string

//go:embed testdata/embedded-5.md
var embeddedMd5 string
