package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// english/semicolon: a semicolon joins sentences that a period keeps apart. The
// repair writes a period. The repair starts a new sentence. Where a period would
// leave a list item as a fragment, the repair rewrites the list instead.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDSemicolon,
		Category: RuleEnglish,
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
