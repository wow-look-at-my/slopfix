// Package fixer is the shape every repair takes, and the list of them.
//
// A Fixer never returns text. It reads a File and hands edits to File.Apply,
// which writes them through the gate the file's parser owns. So a fixer has
// no way to change a byte the parser does not vouch for. Every fixer
// registers itself from its own package's init, and the driver runs the list
// in order.
package fixer

import (
	"fmt"
	"slices"
	"sort"
	"sync"
)

// Kind names the parser that owns a file, and so the gate its edits go through.
type Kind int

const (
	// Source is code, read by the tree-sitter grammar for its extension.
	Source Kind = iota
	// Document is prose, read by the CommonMark parser.
	Document
	// Workflow is a GitHub Actions file, read by the YAML parser.
	Workflow
)

// Fixer is a repair.
type Fixer interface {
	// Name is unique in the registry.
	Name() string
	// Categories are the rule families that select it, as --only names them.
	Categories() []string
	// IDs are the rules whose findings it repairs. It runs when the caller keeps any of them.
	IDs() []string
	// Kinds are the files it repairs.
	Kinds() []Kind
	// Order places it among the fixers of a kind: a lower Order runs earlier.
	Order() int
	// Fix repairs f. It changes f only through Apply and ApplyComments.
	Fix(f *File)
}

var (
	mu       sync.Mutex
	registry []Fixer
)

// Register adds a fixer to the global list. A package calls it from init. A
// second fixer under a name already taken is a programming error.
func Register(fx Fixer) {
	mu.Lock()
	defer mu.Unlock()
	for _, have := range registry {
		if have.Name() == fx.Name() {
			panic(fmt.Sprintf("fixer: %q is registered twice", fx.Name()))
		}
	}
	registry = append(registry, fx)
	sort.SliceStable(registry, func(a, b int) bool {
		if registry[a].Order() != registry[b].Order() {
			return registry[a].Order() < registry[b].Order()
		}
		return registry[a].Name() < registry[b].Name()
	})
}

// All answers every registered fixer, in order.
func All() []Fixer {
	mu.Lock()
	defer mu.Unlock()
	return slices.Clone(registry)
}

// For answers the registered fixers that repair a kind of file, in order.
func For(kind Kind) []Fixer {
	var out []Fixer
	for _, fx := range All() {
		if slices.Contains(fx.Kinds(), kind) {
			out = append(out, fx)
		}
	}
	return out
}

// Named answers the registered fixers with the given names, in registry order.
func Named(names ...string) []Fixer {
	var out []Fixer
	for _, fx := range All() {
		if slices.Contains(names, fx.Name()) {
			out = append(out, fx)
		}
	}
	return out
}

// Spec is a Fixer written as a value, so a package registers one in a few lines.
type Spec struct {
	Label    string
	Families []string
	Rules    []string
	Files    []Kind
	Place    int
	Repair   func(f *File)
}

func (s Spec) Name() string         { return s.Label }
func (s Spec) Categories() []string { return s.Families }
func (s Spec) IDs() []string        { return s.Rules }
func (s Spec) Kinds() []Kind        { return s.Files }
func (s Spec) Order() int           { return s.Place }
func (s Spec) Fix(f *File)          { s.Repair(f) }

// Run applies each fixer the file's selection keeps, in the order given.
func Run(f *File, fixers []Fixer) {
	for _, fx := range fixers {
		if f.selects(fx) {
			fx.Fix(f)
		}
	}
}
