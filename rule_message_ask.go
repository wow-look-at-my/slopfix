package slopfix

import (
	"strings"

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
		Cases:    []RuleCase{{Name: askproperly.ID, Text: "Which store holds the rows?\n"}},
	})
}

// detectAsk answers every question and deferral this message asserts.
func detectAsk(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range askproperly.FindQuestions(c.Text) {
		out = append(out, ste.Finding{
			Line:   lineOf(c.Text, hit.Line),
			ID:     askproperly.ID,
			Rule:   "a decision handed to the reader in prose",
			Detail: hit.Text,
		})
	}
	return out
}

// autofixAsk rewrites each question and each deferral into a statement.
func autofixAsk(c RuleCase) RuleCase {
	c.Text = askproperly.Repair(c.Text)
	return c
}

// lineOf answers the line number, counting from one, of the first line whose
// trimmed text equals the candidate.
func lineOf(text, candidate string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == strings.TrimSpace(candidate) {
			return i + 1
		}
	}
	return 1
}
