// Package lazy loads a grammar the first time a caller asks for it.
package lazy

import (
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// Grammar loads its language once, through the hook the generated parser sets.
type Grammar struct {
	load *func() *ts.Language
	once sync.Once
	lang *ts.Language
}

// New reads load when a caller first asks, because the generated parser sets it in init.
func New(load *func() *ts.Language) *Grammar {
	return &Grammar{load: load}
}

// Language returns the grammar, or nil when the generate step has not run.
func (g *Grammar) Language() *ts.Language {
	g.once.Do(func() {
		if *g.load != nil {
			g.lang = (*g.load)()
		}
	})
	return g.lang
}

// Ready reports whether the generate step has run for this grammar.
func (g *Grammar) Ready() bool {
	return g.Language() != nil
}
