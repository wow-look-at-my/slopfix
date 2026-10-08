package slopfix

import (
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/ste"
)

// laziness/punt: a closing message that leaves the work undone because the work
// is harder, or that asks permission in place of acting. The repair rewrites
// each sentence that carries a tell, so the message owns the work.
func init() {
	RegisterRule(RuleSpec{
		ID:       laziness.ID,
		Category: Rule("laziness"),
		Detect:   detectLaziness,
		Autofix:  autofixLaziness,
		Cases:    []RuleCase{{Name: laziness.ID, Text: "I did not fix the loader.\n"}},
	})
}

// detectLaziness answers every punt this message carries.
func detectLaziness(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range laziness.Check(c.Text) {
		out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: hit.Sentence})
	}
	return out
}

// autofixLaziness rewrites each sentence that carries a tell.
func autofixLaziness(c RuleCase) RuleCase {
	c.Text = laziness.Repair(c.Text)
	return c
}
