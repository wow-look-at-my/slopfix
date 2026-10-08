package slopfix

import (
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/ste"
)

// blame/deflection: a closing message that shifts the work onto another author
// or an earlier time. The repair cuts the sentence that deflects.
func init() {
	RegisterRule(RuleSpec{
		ID:       blamelanguage.ID,
		Category: Rule("blame"),
		Detect:   detectBlame,
		Autofix:  autofixBlame,
		Cases:    []RuleCase{{Name: blamelanguage.ID, Text: "The parser test is red. That failure predates this session.\n"}},
	})
}

// detectBlame answers every banned phrase the case carries.
func detectBlame(c RuleCase) []ste.Finding { return messageFindings(c, blamelanguage.ID) }

// autofixBlame cuts each sentence that deflects.
func autofixBlame(c RuleCase) RuleCase { return messageAutofix(c, blamelanguage.ID) }
