package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/semicolon: STE bans the semicolon. The repair writes a period. The repair
// starts a new sentence. The repair asks for a rewrite by hand only where the
// semicolon joins items that a period would leave as a fragment.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDSemicolon,
		Category: RuleSTE,
		Detect:   detectSemicolon,
		Autofix:  autofixSemicolon,
		Cases:    []RuleCase{{Name: ste.IDSemicolon, Path: "x.md", Text: "The gate reads the file; the tool writes the result.\n"}},
	})
}

// detectSemicolon answers each semicolon the case carries.
func detectSemicolon(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDSemicolon)
}

// autofixSemicolon writes a period and starts a new sentence for every
// semicolon, and answers the case without the finding.
func autofixSemicolon(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDSemicolon)
}
