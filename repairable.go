package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
	"github.com/wow-look-at-my/slopfix/ste"
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
	// The comment rules: one cuts a block back inside its code, and the other
	// says the number in words or takes the sentence that states it.
	commentlength.ID,
	commentnumbers.ID,
))
