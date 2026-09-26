package fixer

import (
	"slices"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// Gate writes edits into text, answering what landed.
type Gate func(text string, edits []edit.Edit, scope edit.Scope) edit.Result

// File is a file under repair. A fixer reads its text and hands edits to
// Apply. The text has no setter: the gates are the only writers.
type File struct {
	// Path decides the parser.
	Path string
	// Kind is the parser that owns the file.
	Kind Kind
	// MaxCommentLines caps a comment block. A cap of nothing turns it off.
	MaxCommentLines int

	text     string
	scope    edit.Scope
	data     Gate
	comments Gate
	wants    func(category string) bool
	keeps    func(id string) bool

	removed  []string
	refused  []edit.Refused
	rewrites int
	notes    []any
}

// Options are what a driver sets on a new File.
type Options struct {
	Kind  Kind
	Scope edit.Scope
	// Data is the gate for an edit that may change what the file means, such.
	Data, Comments Gate
	// Wants and Keeps are the caller's selection. Nil keeps everything.
	Wants func(category string) bool
	Keeps func(id string) bool
	// MaxCommentLines caps a comment block.
	MaxCommentLines int
}

// Open is NewFile with the gate the kind's parser owns. A source file is
// written through its syntax tree and a document through its CommonMark tree.
// A workflow's gates live with the YAML rules, so its caller passes them.
func Open(path, text string, o Options) *File {
	switch o.Kind {
	case Source:
		o.Data = func(text string, edits []edit.Edit, scope edit.Scope) edit.Result {
			return treecomments.Apply(path, text, edits, scope)
		}
		o.Comments = nil
	case Document:
		o.Data, o.Comments = markdown.Apply, nil
	}
	return NewFile(path, text, o)
}

// NewFile opens text for repair.
func NewFile(path, text string, o Options) *File {
	f := &File{
		Path:            path,
		Kind:            o.Kind,
		MaxCommentLines: o.MaxCommentLines,
		text:            text,
		scope:           o.Scope,
		data:            o.Data,
		comments:        o.Comments,
		wants:           o.Wants,
		keeps:           o.Keeps,
	}
	if f.comments == nil {
		f.comments = f.data
	}
	return f
}

// Text is the file as the gates have left it.
func (f *File) Text() string { return f.text }

// Scope bounds, in Text, what the driver's scope bounded.
func (f *File) Scope() edit.Scope { return f.scope }

// Apply writes edits through the file's data gate.
func (f *File) Apply(edits []edit.Edit) edit.Result { return f.through(f.data, edits) }

// ApplyComments writes edits through the gate that proves they changed nothing but comment.
func (f *File) ApplyComments(edits []edit.Edit) edit.Result { return f.through(f.comments, edits) }

func (f *File) through(gate Gate, edits []edit.Edit) edit.Result {
	if len(edits) == 0 || gate == nil {
		return edit.Unchanged(f.text, f.scope)
	}
	res := gate(f.text, edits, f.scope)
	f.text, f.scope = res.Text, res.Scope
	f.removed = append(f.removed, res.Cuts()...)
	f.refused = append(f.refused, res.Refused...)
	return res
}

// Wants reports whether the caller selected a rule family.
func (f *File) Wants(category string) bool { return f.wants == nil || f.wants(category) }

// Keeps reports whether the caller selected a rule.
func (f *File) Keeps(id string) bool { return f.keeps == nil || f.keeps(id) }

// selects reports whether the caller's selection reaches a fixer.
func (f *File) selects(fx Fixer) bool {
	wanted := slices.ContainsFunc(fx.Categories(), f.Wants)
	return wanted && slices.ContainsFunc(fx.IDs(), f.Keeps)
}

// Rewrote counts rewrites the english table made.
func (f *File) Rewrote(n int) { f.rewrites += n }

// RemovedOnce notes a quote the report must carry, once.
func (f *File) RemovedOnce(quote string) {
	if !slices.Contains(f.removed, quote) {
		f.removed = append(f.removed, quote)
	}
}

// Note keeps a finding a fixer could not repair, for the driver to report.
func (f *File) Note(v any) { f.notes = append(f.notes, v) }

// Report is what the fixers did to a File.
type Report struct {
	Removed  []string
	Refused  []edit.Refused
	Rewrites int
	Notes    []any
}

// Report answers what the fixers did.
func (f *File) Report() Report {
	return Report{Removed: f.removed, Refused: f.refused, Rewrites: f.rewrites, Notes: f.notes}
}
