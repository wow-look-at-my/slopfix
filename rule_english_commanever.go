package slopfix

import (
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
)

// english/comma-never: ", never" contrasts a clause with the one before it, and
// the contrast reads as a claim the sentence does not make. The repair writes
// ", not" before a phrase, and ", and never" or ", do not" where the words
// after the comma are a verb.
func init() {
	RegisterRule(RuleSpec{
		ID:       english.IDCommaNever,
		Category: RuleEnglish,
		Detect:   detectCommaNever,
		Autofix:  autofixCommaNever,
		Cases: []RuleCase{
			{Name: english.IDCommaNever, Path: "x.md", Text: "The gate reads the file, never the tool.\n"},
			{
				Name: "a verb in -s that closes the sentence", Path: "x.md",
				Text: "A missing validation layer fails the run, never skips.\n",
				Want: "A missing validation layer fails the run, and never skips.\n",
			},
		},
	})
}

// detectCommaNever answers each ", never" contrast the case carries.
func detectCommaNever(c RuleCase) []ste.Finding {
	return caseFindings(c, english.IDCommaNever)
}

// autofixCommaNever writes ", not" or the verb form the contrast needs, and
// answers the case without the finding.
func autofixCommaNever(c RuleCase) RuleCase {
	return caseAutofix(c, english.IDCommaNever)
}
