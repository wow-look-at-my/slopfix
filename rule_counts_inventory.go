package slopfix

import (
	"github.com/wow-look-at-my/slopfix/counts"
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

// detectInventory answers the inventory-count rule's findings from the document
// substrate its repair reads.
func detectInventory(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range counts.Check(c.Text) {
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
