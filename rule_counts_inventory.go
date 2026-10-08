package slopfix

import (
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
)

// counts/inventory-count: a document that counts what is here. The repair cuts
// the cardinal out of the sentence, or writes words that state no figure.
func init() {
	RegisterRule(RuleSpec{
		ID:       counts.ID,
		Category: RuleCounts,
		Detect:   detectInventoryCount,
		Autofix:  autofixInventoryCount,
		Cases:    []RuleCase{{Name: counts.ID, Path: "x.md", Text: "This project has 20 rules.\n"}},
	})
}

// detectInventoryCount answers the inventory-count rule's findings from the
// document substrate its repair reads.
func detectInventoryCount(c RuleCase) []ste.Finding {
	return detectInventory(c)
}

// autofixInventoryCount cuts the cardinal out of the sentence, or writes words
// that state no figure.
func autofixInventoryCount(c RuleCase) RuleCase {
	return caseAutofix(c, counts.ID)
}

// detectInventory answers the inventory-count rule's findings from the
// documents its repair reads: each document of the repository, and the message.
func detectInventory(c RuleCase) []ste.Finding {
	var docs []string
	for _, path := range fixtureFiles(c) {
		if kindOf(path, readFixture(path)) == fixer.Document {
			docs = append(docs, readFixture(path))
		}
	}
	if c.Text != "" {
		docs = append(docs, c.Text)
	}
	var out []ste.Finding
	for _, doc := range docs {
		out = append(out, inventoryFindings(doc)...)
	}
	return out
}

// inventoryFindings answers each inventory count in one document.
func inventoryFindings(doc string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range counts.Check(doc) {
		out = append(out, ste.Finding{
			Line:   hit.LineNo,
			ID:     counts.ID,
			Rule:   "a stated count goes stale when the set changes",
			Detail: hit.Phrase,
			Fix:    "Describe what is there and let the reader count.",
		})
	}
	return out
}
