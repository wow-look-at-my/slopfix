package ste

import "github.com/wow-look-at-my/go-containers/set"

// Repairs names the rules FixSelected rewrites. A test drives Fix over a
// sample of every rule, so this cannot drift from the repair pass.
var Repairs = set.Of(IDContraction, IDModal, IDSemicolon, IDCommaSplice)
