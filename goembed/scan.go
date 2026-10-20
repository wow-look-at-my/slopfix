package goembed

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/walktree"
)

// found is a string expression in a Go file that holds another language.
type found struct {
	// start and end are the byte span of the expression.
	start, end int
	line       int
	value      string
	lang       Lang
	// spec is the package-level var or const that holds the string alone, or nil.
	spec *ast.ValueSpec
	// decl holds spec, when spec is its only entry.
	decl *ast.GenDecl
	// fn names the function the expression sits in, or "" at package level.
	fn string
}

// parse reads src as a Go file, comments included.
func parse(src string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	return fset, file, err
}

// literal answers the value of an expression built only of string literals
// and the + operator.
func literal(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		return v, err == nil
	case *ast.ParenExpr:
		return literal(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		x, ok := literal(e.X)
		if !ok {
			return "", false
		}
		y, ok := literal(e.Y)
		return x + y, ok
	}
	return "", false
}

// scan answers each string expression of src that holds another language,
// where the repair can put a variable in its place.
func scan(path, src string) []found {
	if !strings.ContainsAny(src, "`\"") {
		return nil
	}
	fset, file, err := parse(src)
	if err != nil {
		return nil
	}
	consts := constReads(path, file)
	var out []found
	add := func(expr ast.Expr, value string, spec *ast.ValueSpec, decl *ast.GenDecl, fn string) {
		lang, ok := Classify(value)
		if !ok {
			return
		}
		out = append(out, found{
			start: fset.Position(expr.Pos()).Offset,
			end:   fset.Position(expr.End()).Offset,
			line:  fset.Position(expr.Pos()).Line,
			value: value, lang: lang, spec: spec, decl: decl, fn: fn,
		})
	}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok != token.VAR && d.Tok != token.CONST {
				continue
			}
			for _, s := range d.Specs {
				spec := s.(*ast.ValueSpec)
				// A const or a typed spec of one name takes the file whole, so the
				// spec becomes the embedded var.
				// A const that another const reads must stay a const, so no var takes its place.
				if len(spec.Names) == 1 && len(spec.Values) == 1 && embeddable(spec.Type) {
					if v, ok := literal(spec.Values[0]); ok {
						if d.Tok == token.CONST && consts.Contains(spec.Names[0].Name) {
							continue
						}
						var whole *ast.GenDecl
						if len(d.Specs) == 1 {
							whole = d
						}
						add(spec.Values[0], v, spec, whole, "")
						continue
					}
				}
				if d.Tok == token.CONST {
					continue
				}
				for _, value := range spec.Values {
					walk(value, spec.Names[0].Name, add)
				}
			}
		case *ast.FuncDecl:
			if d.Body != nil {
				walk(d.Body, d.Name.Name, add)
			}
		}
	}
	return out
}

// constReads answers each name a const declaration reads, in file and in the
// other Go files of its directory.
func constReads(path string, file *ast.File) set.Set[string] {
	names := set.New[string]()
	read := func(f *ast.File) {
		for _, d := range f.Decls {
			ast.Inspect(d, func(n ast.Node) bool {
				g, ok := n.(*ast.GenDecl)
				if !ok {
					return true
				}
				if g.Tok != token.CONST {
					return g.Tok != token.IMPORT
				}
				for _, s := range g.Specs {
					for _, v := range s.(*ast.ValueSpec).Values {
						ast.Inspect(v, func(n ast.Node) bool {
							if id, ok := n.(*ast.Ident); ok {
								names.Add(id.Name)
							}
							return true
						})
					}
				}
				return false
			})
		}
	}
	read(file)
	dir := filepath.Dir(path)
	entries, err := walktree.List(dir)
	if err != nil {
		return names
	}
	for _, e := range entries {
		other := filepath.Join(dir, e.Name())
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || other == filepath.Clean(path) {
			continue
		}
		if b, err := os.ReadFile(other); err == nil {
			if _, f, err := parse(string(b)); err == nil {
				read(f)
			}
		}
	}
	return names
}

// embeddable reports whether a spec of this type can be a //go:embed var.
func embeddable(typ ast.Expr) bool {
	if typ == nil {
		return true
	}
	ident, ok := typ.(*ast.Ident)
	return ok && ident.Name == "string"
}

// walk hands add each largest string expression under node that a variable
// can replace: one outside a const declaration, a struct tag and an import.
func walk(node ast.Node, fn string, add func(ast.Expr, string, *ast.ValueSpec, *ast.GenDecl, string)) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GenDecl:
			return n.Tok != token.CONST && n.Tok != token.IMPORT
		case *ast.Field:
			// A tag is a literal by grammar, so only the type is read.
			if n.Type != nil {
				walk(n.Type, fn, add)
			}
			return false
		case *ast.CaseClause, *ast.CommClause:
			return true
		case ast.Expr:
			if v, ok := literal(n); ok {
				if _, isLit := n.(*ast.BasicLit); isLit || isConcat(n) {
					add(n, v, nil, nil, fn)
				}
				return false
			}
		}
		return true
	})
}

// isConcat reports whether expr joins literals with +.
func isConcat(expr ast.Expr) bool {
	_, ok := expr.(*ast.BinaryExpr)
	return ok
}
