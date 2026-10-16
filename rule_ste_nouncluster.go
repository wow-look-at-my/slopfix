package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/noun-cluster: more nouns stand together than a reader can hold apart. The
// repair writes the last noun as the head and the nouns in front of it as its
// prepositional complement.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDNounCluster,
		Category: RuleSTE,
		Detect:   detectNounCluster,
		Autofix:  autofixNounCluster,
		Cases: []RuleCase{
			{Name: ste.IDNounCluster, Path: "w.md", Text: "The gate cache lookup table stopped.\n", Want: "The table of the gate cache lookup stopped.\n"},
			{
				Name: "a cluster that ends in a product name", Path: "w.md",
				Text: "It answers the Claude Code hook events.\n",
				Want: "It answers the events of the Claude Code hook.\n",
			},
			{
				Name: "a title-cased cluster", Path: "w.md",
				Text: "It reads the Gate Cache Lookup Table.\n",
				Want: "It reads the Table of the Gate Cache Lookup.\n",
			},
			{
				Name: "an identifier in front of a named standard", Path: "w.md",
				Text: "Its prose follows ASD-STE100 Simplified Technical English.\n",
				Want: "Its prose follows Simplified Technical English (ASD-STE100).\n",
			},
		},
	})
}

// detectNounCluster answers every noun cluster this case reports.
func detectNounCluster(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDNounCluster)
}

// autofixNounCluster writes the nouns in front of the head as its complement.
func autofixNounCluster(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDNounCluster)
}
