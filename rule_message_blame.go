package slopfix

import (
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/ste"
)

// blame/deflection: a closing message that shifts responsibility for code in this org's own repositories onto another author or an earlier time. The repair rewrites each phrase that shifts the work
// away, so the message owns the defect.
func init() {
	RegisterRule(RuleSpec{
		ID:       blamelanguage.ID,
		Category: Rule("blame"),
		Detect:   detectBlame,
		Autofix:  autofixBlame,
		Cases:    []RuleCase{{Name: blamelanguage.ID, Text: "That failure predates this session.\n"}},
	})
}

// detectBlame answers every banned phrase this message carries.
func detectBlame(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range blamelanguage.Check(c.Text) {
		detail := hit.Phrase
		if detail == "" {
			detail = hit.Sentence
		}
		out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: detail})
	}
	return out
}

// autofixBlame rewrites each phrase that shifts the work onto another author.
func autofixBlame(c RuleCase) RuleCase {
	c.Text = blamelanguage.Repair(c.Text)
	return c
}
