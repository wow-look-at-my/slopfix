package typescript

import (
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// load is set by the parser.gen.go the generate step writes. The tables are
// data, generated at build time, so this half compiles without them and a
// consumer resolving this module from the proxy still gets a package.
var (
	load      func() *ts.Language
	loadOnce  sync.Once
	generated *ts.Language
)

// Language returns the grammar, decoding its tables when a parse needs them.
// It returns nil when the generate step has not run.
//
// A module zip carries no generated file, so a consumer that resolves this
// module from the proxy has no tables and never can. A panic there takes down
// a whole toolchain over a single language it was not asked about. The caller
// says what it is skipping instead.
func Language() *ts.Language {
	loadOnce.Do(func() {
		if load != nil {
			generated = load()
		}
	})
	return generated
}

// Ready reports whether the generate step has run for this grammar.
func Ready() bool {
	return Language() != nil
}
