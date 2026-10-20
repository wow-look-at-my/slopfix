package goembed

import (
	"errors"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
)

// Created is a file the repair writes, by the path its Go file's path names it.
type Created = fixer.Created

// plan is the whole repair of one Go file: the edits and the files they embed.
type plan struct {
	edits   []edit.Edit
	created []Created
}

// identifier finds each Go identifier in a text.
var identifier = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)

// taken answers every identifier the Go files beside path use, so a new var
// takes a name no file of the package holds.
func taken(path, src string) set.Set[string] {
	names := set.New[string]()
	read := func(text string) {
		for _, name := range identifier.FindAllString(text, -1) {
			names.Add(name)
		}
	}
	read(src)
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || filepath.Join(dir, e.Name()) == filepath.Clean(path) {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			read(string(b))
		}
	}
	return names
}

// unheld reports whether a file can go at path: nothing is there, or the same
// text is.
func unheld(path, text string) bool {
	held, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return err == nil && string(held) == text
}

// kebab writes a Go name as the stem of a file name.
// An initialism stays one word: TestAPackageJSONCase is test-a-package-json-case.
func kebab(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('-')
			}
		}
		r = unicode.ToLower(r)
		if r == '_' {
			r = '-'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// stem answers the name of the function a hoisted string sits in, which
// names its file.
func stem(f found) string {
	base := f.fn
	if base == "" || base == "init" || base == "_" {
		base = "embedded"
	}
	runes := []rune(base)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// varName answers the name a hoisted string takes: its stem, then the word
// for its language.
func varName(f found) string {
	word := []rune(strings.TrimPrefix(f.lang.Ext, "."))
	word[0] = unicode.ToUpper(word[0])
	return stem(f) + string(word)
}

// planFor answers the repair of src, which is headed for path.
func planFor(path, src string) (plan, bool) {
	hits := scan(path, src)
	if len(hits) == 0 {
		return plan{}, false
	}
	fset, file, err := parse(src)
	if err != nil {
		return plan{}, false
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }
	dir := filepath.Dir(path)
	prefix := ""
	if strings.HasSuffix(path, "_test.go") {
		prefix = "testdata/"
	}
	names := taken(path, src)
	files := set.New[string]()
	place := func(stem, ext, text string) (string, bool) {
		for n := 1; n <= 100; n++ {
			rel := prefix + stem
			if n > 1 {
				rel += "-" + strconv.Itoa(n)
			}
			rel += ext
			if !files.Contains(rel) && unheld(filepath.Join(dir, filepath.FromSlash(rel)), text) {
				files.Add(rel)
				return rel, true
			}
		}
		return "", false
	}
	var p plan
	var tail strings.Builder
	for _, f := range hits {
		if f.spec != nil {
			name := f.spec.Names[0].Name
			rel, ok := place(kebab(name), f.lang.Ext, f.value)
			if !ok {
				continue
			}
			directive := "//go:embed " + rel + "\n"
			switch {
			case f.decl != nil:
				p.edits = append(p.edits, edit.Edit{Start: offset(f.decl.Pos()), End: offset(f.decl.End()), Text: directive + "var " + name + " string", Cut: []string{f.value}})
			case declTok(file, f.spec) == token.VAR:
				p.edits = append(p.edits, edit.Edit{Start: offset(f.spec.Pos()), End: offset(f.spec.End()), Text: directive + name + " string", Cut: []string{f.value}})
			default:
				// A const group cannot hold a var, so the spec leaves the group.
				p.edits = append(p.edits, edit.Edit{Start: offset(f.spec.Pos()), End: offset(f.spec.End()), Text: "", Cut: []string{f.value}})
				tail.WriteString("\n" + directive + "var " + name + " string\n")
			}
			p.created = append(p.created, Created{Path: filepath.Join(dir, filepath.FromSlash(rel)), Text: f.value})
			continue
		}
		name := varName(f)
		for n := 2; names.Contains(name); n++ {
			name = varName(f) + strconv.Itoa(n)
		}
		rel, ok := place(kebab(stem(f)), f.lang.Ext, f.value)
		if !ok {
			continue
		}
		names.Add(name)
		p.edits = append(p.edits, edit.Edit{Start: f.start, End: f.end, Text: name, Cut: []string{f.value}})
		tail.WriteString("\n//go:embed " + rel + "\nvar " + name + " string\n")
		p.created = append(p.created, Created{Path: filepath.Join(dir, filepath.FromSlash(rel)), Text: f.value})
	}
	if len(p.created) == 0 {
		return plan{}, false
	}
	if tail.Len() > 0 {
		end := len(src)
		text := tail.String()
		if !strings.HasSuffix(src, "\n") {
			text = "\n" + text
		}
		p.edits = append(p.edits, edit.Edit{Start: end, End: end, Text: text})
	}
	if !importsEmbed(file) {
		at := offset(file.Name.End())
		p.edits = append(p.edits, edit.Edit{Start: at, End: at, Text: "\n\nimport _ \"embed\""})
	}
	return p, true
}

// declTok answers the token of the declaration that holds spec.
func declTok(file *ast.File, spec *ast.ValueSpec) token.Token {
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok {
			for _, s := range g.Specs {
				if s == spec {
					return g.Tok
				}
			}
		}
	}
	return token.ILLEGAL
}

// importsEmbed reports whether the file imports the embed package.
func importsEmbed(file *ast.File) bool {
	for _, spec := range file.Imports {
		if p, err := strconv.Unquote(spec.Path.Value); err == nil && p == "embed" {
			return true
		}
	}
	return false
}
