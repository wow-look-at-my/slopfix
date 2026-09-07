package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Repairable reports whether slopfix repairs the defect a finding names.
//
// It answers about the DEFECT rather than about the rule that found it. A stale
// count is reported by the prose rules and repaired by the counts rule, under a
// different ID. A caller told the finding is unrepairable would report it, and
// the reader then gets the repair and the report on the same write.
//
// That is what the property exists to prevent. A reporting caller takes the
// findings this calls false. A repairing caller takes the rest. Neither carries
// a list, and neither has to agree with the other about one.
func Repairable(id string) bool {
	return repairable.Contains(id)
}

// repairable is derived rather than declared: each entry names a repair that
// exists in this repository, and a test drives every one of them.
var repairable = ste.Repairs.Clone().Union(set.Of(
	// The wrap join.
	IDHardWrap,
	// The counts rule cuts the cardinal out of the same sentence the prose
	// rules report a stale count in.
	ste.IDStaleCount,
	IDInventoryCount,
))
