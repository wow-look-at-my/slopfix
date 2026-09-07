package ste

import "github.com/wow-look-at-my/go-containers/set"

// Repairs names the rules FixSelected rewrites. The rest are reported and left
// for a writer, because no rewrite can choose what the sentence meant.
//
// A test drives Fix over a sample of every rule and asserts the two agree, so
// this cannot drift from what the repair pass really does.
var Repairs = set.Of(IDContraction, IDModal, IDSemicolon, IDCommaSplice)
