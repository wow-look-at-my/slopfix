// Package goembed finds a string in a Go file that holds the source of
// another language. Such text belongs in a file of its own type, and the Go
// program reads it with //go:embed.
package goembed

import (
	"go/parser"
	"go/token"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
)

// ID names the rule.
const ID = "go/foreign-source"

// Category is the family --only names.
const Category = "go"

// AllIDs names every rule in the category.
var AllIDs = set.Of(ID)

// Judges reports whether the rule reads a file of that name.
func Judges(path string) bool { return strings.HasSuffix(path, ".go") }

// CheckPath reports each string of a Go file that holds another language.
func CheckPath(path, src string) []ste.Finding {
	if !Judges(path) {
		return nil
	}
	var out []ste.Finding
	for _, f := range scan(src) {
		first, _, _ := strings.Cut(strings.TrimSpace(f.value), "\n")
		out = append(out, ste.Finding{
			Line:   f.line,
			ID:     ID,
			Rule:   "a Go string holds " + f.lang.Name + " source",
			Detail: first,
			Fix:    "Move the text into a " + f.lang.Ext + " file and read it with //go:embed. `slopfix fix` does this.",
		})
	}
	return out
}

// gate writes an edit only when it is one planFor derives from src, and holds
// only when the result parses as Go.
func gate(path string) fixer.Gate {
	return func(src string, edits []edit.Edit, scope edit.Scope) edit.Result {
		p, _ := planFor(path, src)
		reach := func(e edit.Edit) string {
			for _, o := range p.edits {
				if o.Start == e.Start && o.End == e.End && o.Text == e.Text {
					return ""
				}
			}
			return "it is not the move of a string into an embedded file"
		}
		holds := func(text string) bool {
			_, err := parser.ParseFile(token.NewFileSet(), path, text, parser.ParseComments)
			return err == nil
		}
		return edit.Gate(src, edits, scope, reach, holds)
	}
}

// repair moves each string that holds another language into its own file. A
// driver that writes the Go file alone gets no change, because the var would
// embed a file nobody wrote.
func repair(f *fixer.File) {
	if !Judges(f.Path) || !f.Creates() {
		return
	}
	p, ok := planFor(f.Path, f.Text())
	if !ok {
		return
	}
	if res := f.ApplyThrough(gate(f.Path), p.edits); len(res.Applied) != len(p.edits) {
		return
	}
	for _, c := range p.created {
		f.Create(c.Path, c.Text)
	}
}

func init() {
	fixer.Register(fixer.Spec{
		Label:    ID,
		Families: []string{Category},
		Rules:    []string{ID},
		Files:    []fixer.Kind{fixer.Source},
		Place:    900,
		Repair:   repair,
	})
}
