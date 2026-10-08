package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/postdeterminer: a cardinal after a determiner, as in "all the rules".
// The repair writes the determiner's own number word, or drops the cardinal.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDPostdeterminer,
		Category: RuleSTE,
		Detect:   detectPostdeterminer,
		Autofix:  autofixPostdeterminer,
		Cases:    []RuleCase{{Name: ste.IDPostdeterminer, Path: "x.md", Text: "All the three rules apply here.\n"}},
	})
}

// detectPostdeterminer answers each cardinal that follows a determiner in the
// case.
func detectPostdeterminer(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDPostdeterminer)
}

// autofixPostdeterminer writes the determiner's own number word or drops the
// cardinal, and answers the case without the finding.
func autofixPostdeterminer(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDPostdeterminer)
}
