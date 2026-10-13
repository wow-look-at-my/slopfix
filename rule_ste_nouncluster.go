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
			{Name: ste.IDNounCluster, Path: "w.md", Text: "The gate cache lookup table stopped.\n"},
			{Name: "a title and a verb stay out of a cluster", Path: "w.md", Text: titlesCase, Want: titlesWant},
		},
	})
}

// titlesCase holds a real cluster beside words the rule must leave alone. A
// verb the tagger reads as a plural noun ("center depth sorts triangles"), and
// paper titles in title case, which name a work.
const titlesCase = "The gate cache lookup table stopped. Center depth sorts triangles, and the paper reports popping from that.\n" +
	"\n" +
	"- [Triangle Splatting for Real-Time Radiance Field Rendering](https://arxiv.org/abs/2505.19175)\n" +
	"- [Efficient Perspective-Correct Gaussian Splatting Using Hybrid Transparency](https://arxiv.org/abs/2410.08129)\n"

const titlesWant = "The table of the gate cache lookup stopped. Center depth sorts triangles, and the paper reports popping from that.\n" +
	"\n" +
	"- [Triangle Splatting for Real-Time Radiance Field Rendering](https://arxiv.org/abs/2505.19175)\n" +
	"- [Efficient Perspective-Correct Gaussian Splatting Using Hybrid Transparency](https://arxiv.org/abs/2410.08129)\n"

// detectNounCluster answers every noun cluster this case reports.
func detectNounCluster(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDNounCluster)
}

// autofixNounCluster writes the nouns in front of the head as its complement.
func autofixNounCluster(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDNounCluster)
}
