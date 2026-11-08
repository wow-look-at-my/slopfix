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
			{Name: "frame", Path: "x.md", Text: embeddedFrame},
			{Name: "a list of statements is prose", Path: "x.md", Text: embeddedMd2, Unchanged: true},
			{Name: "a table that describes each row is content", Path: "x.md", Text: embeddedMd3, Unchanged: true},
			{Name: "a reference table is content", Path: "x.md", Text: embeddedTableReference, Unchanged: true},
			{Name: "a fence is content, not a list", Path: "x.md", Text: embeddedMd4, Unchanged: true},
			{Name: "a list of items of several sentences is prose", Path: "x.md", Text: embeddedProseItems, Unchanged: true},
			{Name: "a table of names goes", Path: "x.md", Text: embeddedTableNames, Want: listKept},
			{Name: "a mixed table loses its rows of names", Path: "x.md", Text: embeddedTableMixed, Want: listKept + "\n| Command | Description |\n|---|---|\n| `check` | Reads each file and reports each finding. |\n"},
			{Name: "a cell loses its enumeration", Path: "x.md", Text: embeddedTableCell, Want: listKept + "\n| Command | Description |\n|---|---|\n| `check` | Reads each file. |\n| `fix` | Repairs each file. |\n"},
			{Name: "a list of names goes", Path: "x.md", Text: embeddedListNames, Want: listKept},
			{Name: "an item loses its enumeration", Path: "x.md", Text: embeddedListItemCut, Want: listKept + "\n- `check` reads a file.\n- `fix` repairs a file.\n"},
			{Name: "a sentence loses its parenthesis of names", Path: "x.md", Text: embeddedAside, Want: listKept + "\nThe tool skips hidden paths on each run.\n"},
			{Name: "a sentence loses its run behind a lead-in", Path: "x.md", Text: embeddedLeadIn, Want: listKept + "\nThe tool skips hidden paths.\n"},
			{Name: "a row goes when a cell is only names", Path: "x.md", Text: embeddedTableCellOnly, Want: listKept + "\n| Command | Description |\n|---|---|\n| `check` | Reads each file. |\n"},
			{Name: "a pointer sentence goes with its table", Path: "x.md", Text: embeddedPointer, Want: listKept},
			{Name: "an item goes when its enumeration is its content", Path: "x.md", Text: embeddedItemOnlyList, Want: listKept + "\n- `check` reads a file.\n"},
			{Name: "split", Path: "x.md", Text: embeddedMd5},
			{Name: "comment", Path: "x.go", Text: "// x.go holds mode.\n\npackage x\n\n// The modes are `fast`, `slow` and `safe`.\nconst mode = 1\n"},
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

// listFindings answers each enumeration in one document. It reports at warning
// severity, so a document that names a set is named rather than failed, and
// `slopfix fix` is what removes it.
func listFindings(doc string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range counts.FindLists(doc) {
		out = append(out, ste.Finding{
			Line:     hit.LineNo,
			ID:       counts.IDList,
			Rule:     listRule,
			Detail:   hit.Detail,
			Fix:      listFix,
			Severity: ste.SeverityWarning,
		})
	}
	return out
}

// commentListFindings answers each enumeration in the comments of a source file or a workflow.
func commentListFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range commentfix.CheckLists(path, content) {
		out = append(out, ste.Finding{
			Line:     hit.Line,
			ID:       counts.IDList,
			Rule:     listRule,
			Detail:   hit.Sentence,
			Fix:      listFix,
			Severity: ste.SeverityWarning,
		})
	}
	return out
}

//go:embed testdata/embedded-2.md
var embeddedMd2 string

//go:embed testdata/embedded-3.md
var embeddedMd3 string

//go:embed testdata/embedded-4.md
var embeddedMd4 string

//go:embed testdata/embedded-5.md
var embeddedMd5 string

//go:embed testdata/inventory-frame.md
var embeddedFrame string

//go:embed testdata/inventory-prose-items.md
var embeddedProseItems string

//go:embed testdata/inventory-table-names.md
var embeddedTableNames string

//go:embed testdata/inventory-table-reference.md
var embeddedTableReference string

//go:embed testdata/inventory-table-mixed.md
var embeddedTableMixed string

//go:embed testdata/inventory-table-cell.md
var embeddedTableCell string

//go:embed testdata/inventory-list-names.md
var embeddedListNames string

//go:embed testdata/inventory-list-item-cut.md
var embeddedListItemCut string

//go:embed testdata/inventory-aside.md
var embeddedAside string

//go:embed testdata/inventory-lead-in.md
var embeddedLeadIn string

//go:embed testdata/inventory-table-cell-only.md
var embeddedTableCellOnly string

//go:embed testdata/inventory-pointer.md
var embeddedPointer string

//go:embed testdata/inventory-item-only-list.md
var embeddedItemOnlyList string

// listKept is what each fixture keeps above the list it tests.
const listKept = "# Tool\n\nThe tool reads prose.\n"
