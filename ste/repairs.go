package ste

import "github.com/wow-look-at-my/go-containers/set"

// Repairs names the rules FixSelected rewrites, pinned by a test over every rule.
var Repairs = set.Of(IDContraction, IDModal, IDSemicolon, IDCommaSplice)
