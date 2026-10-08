package slopfix

import (
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/ste"
)

// laziness/punt: a closing message that leaves the work undone because the work
// is harder, or that asks permission in place of acting. The repair cuts each
// sentence that carries a tell.
func init() {
	RegisterRule(RuleSpec{
		ID:       laziness.ID,
		Category: Rule("laziness"),
		Detect:   detectLaziness,
		Autofix:  autofixLaziness,
		Cases: []RuleCase{
			{Name: laziness.ID, Text: "The loader reads the cache. That is out of scope.\n"},
			{Name: laziness.ID + "/defect", Text: "I found an off-by-one in the retry loop and left it alone.\n"},
		},
	})
}

// detectLaziness answers every punt the case carries.
func detectLaziness(c RuleCase) []ste.Finding { return messageFindings(c, laziness.ID) }

// autofixLaziness cuts each sentence that carries a tell.
func autofixLaziness(c RuleCase) RuleCase { return messageAutofix(c, laziness.ID) }
