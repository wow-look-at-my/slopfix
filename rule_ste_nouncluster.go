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
			{Name: "default is a verb after a plural subject, so no noun cluster stands there", Path: "w.md", Unchanged: true, Text: "Cache paths default ON wherever the keys exist.\n"},
			{Name: "become is a verb after a reduced relative, so no noun cluster stands there", Path: "w.md", Unchanged: true, Text: "Move the fields into the struct: fields computed in setup become plain members plus an assignment.\n"},
			{Name: ste.IDNounCluster, Path: "w.md", Text: "The gate cache lookup table stopped.\n", Want: "The table of the gate cache lookup stopped.\n"},
			{Name: "a verb stays out of a cluster", Path: "w.md", Text: verbCase, Want: verbWant},
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

// verbCase holds a real cluster beside a verb the tagger reads as a plural noun ("center depth sorts triangles").
const verbCase = "The gate cache lookup table stopped. Center depth sorts triangles.\n"

const verbWant = "The table of the gate cache lookup stopped. Center depth sorts triangles.\n"

// detectNounCluster answers every noun cluster this case reports.
func detectNounCluster(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDNounCluster)
}

// autofixNounCluster writes the nouns in front of the head as its complement.
func autofixNounCluster(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDNounCluster)
}
