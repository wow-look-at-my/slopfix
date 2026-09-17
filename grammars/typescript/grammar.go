package typescript

import (
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// load is set by the parser.gen.go the generate step writes, so this half
// compiles without the tables.
var (
	load      func() *ts.Language
	loadOnce  sync.Once
	generated *ts.Language
)

// Language returns the grammar, or nil when the generate step has not run.
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
