package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// Repairable reports whether slopfix repairs the DEFECT a finding names,
// rather than the rule that found it.
func Repairable(id string) bool {
	return repairable.Contains(id)
}

// repairable is derived: each entry names a repair a test drives.
var repairable = ste.Repairs.Clone().Union(set.Of(
	// The wrap join.
	IDHardWrap,
	// The counts rule cuts the cardinal out of the same sentence the prose
	ste.IDStaleCount,
	IDInventoryCount,
	// The comment rules: a block is cut back inside its code, and a number is
	// said in words or loses the sentence stating it.
	commentlength.ID,
	commentnumbers.ID,
	// The workflow rules: the gate loses its continue-on-error, the comment
	// block folds to a line, and the shadowing job is renamed.
	workflow.IDNeuteredGate,
	workflow.IDCommentBlock,
	workflow.IDAllBuildsJob,
))
