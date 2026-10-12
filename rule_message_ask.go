package slopfix

import (
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/ste"
)

// ask/prose-decision: a closing message that puts a decision to the reader in
// prose, or defers one without AskUserQuestion.
func init() {
	RegisterRule(RuleSpec{
		ID:       askproperly.ID,
		Category: Rule("ask"),
		Detect:   detectAsk,
		Autofix:  autofixAsk,
		Cases:    []RuleCase{{Name: askproperly.ID, Text: "The cache holds the rows. Which store holds the rows?\n"}},
	})
}

// detectAsk answers every question and deferral the case carries.
func detectAsk(c RuleCase) []ste.Finding { return messageFindings(c, askproperly.ID) }

// autofixAsk cuts each question and each deferral.
func autofixAsk(c RuleCase) RuleCase { return messageAutofix(c, askproperly.ID) }
