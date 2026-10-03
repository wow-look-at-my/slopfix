package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// EveryID names every rule a category holds, the repository rules included.
func EveryID() set.Set[string] {
	ids := set.New[string]()
	for _, rule := range AllRules {
		for id := range IDsFor(rule).All() {
			ids.Add(id)
		}
	}
	return ids
}

// Repairable reports whether slopfix repairs the DEFECT a finding names,
// rather than the rule that found it.
func Repairable(id string) bool {
	return repairable.Contains(id)
}

// repairable is derived: each entry names a repair a test drives.
var repairable = ste.Repairs.Clone().Union(set.Of(
	// The wrap join, and the division of a long block.
	IDHardWrap,
	IDLongBlock,
	// The counts rule cuts the cardinal out of the same sentence the prose
	ste.IDStaleCount,
	IDInventoryCount,
	// The comment rules: a block fits its code, a number is said in words, and a comment says something whole.
	commentfix.IDLength,
	commentfix.ID,
	commentfix.IDTail,
	// The workflow rules: the gate, the comment block, the shadowing job name.
	workflow.IDNeuteredGate,
	workflow.IDCommentBlock,
	workflow.IDAllBuildsJob,
	workflow.IDTestInYAML,
	workflow.IDEnvIndirection,
	workflow.IDPushTags,
	workflow.IDOrgActionRef,
	// The v parameter comes out of a download URL.
	pins.ID,
	// ", never" becomes ", not".
	english.IDCommaNever,
)).Union(tombstones.AllIDs()).Union(RepoIDs.Difference(ReportOnly))
