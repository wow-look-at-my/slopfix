// grepjson.go rewrites a grep over JSON files into a jq query.
//
// grep reads a JSON file as lines of text. It matches escape sequences and
// punctuation, and it prints a fragment with no path to the value. jq tests the
// keys and the decoded values instead:
//
//	JSON Lines: each matching record, whole and compact, as grep prints a line
//	document:   each matching leaf as `.path.to[0].key = value`
//
// A grep with a flag outside grepjson.xml, a pattern that is not literal, or
// an operand that is not JSON stays as written.
package bashclean

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"
)

// grepCall is a grep this rule can translate.
type grepCall struct {
	patterns []string
	syntax   regexSyntax
	icase    bool
	smart    bool
	invert   bool
	word     bool
	whole    bool
	count    bool
	numbers  bool
	names    int
	files    []*syntax.Word
}

var reMeta = regexp.MustCompile(`[\\^$.|?*+()\[\]{}]`)

// translatePattern writes a pattern as Oniguruma reads it. jq links Oniguruma
// with its Perl syntax, so a POSIX pattern needs translation.
func translatePattern(syn regexSyntax, p string) (string, bool) {
	switch syn {
	case reFixed:
		return reMeta.ReplaceAllString(p, `\$0`), true
	case rePerl:
		return p, true
	}
	return posixToOnig(p, syn == reBasic)
}

// onigRegex builds the regex jq's `test` reads. A newline in a pattern
// separates alternatives, as it does for grep.
func (g grepCall) onigRegex() (string, bool) {
	var alts []string
	for _, p := range g.patterns {
		for _, line := range strings.Split(p, "\n") {
			re, ok := translatePattern(g.syntax, line)
			if !ok {
				return "", false
			}
			alts = append(alts, "(?:"+re+")")
		}
	}
	re := strings.Join(alts, "|")
	switch {
	case g.whole:
		re = `\A(?:` + re + `)\z`
	case g.word:
		re = `(?<!\w)(?:` + re + `)(?!\w)`
	}
	return re, true
}

// caseFlag is the jq `test` flag. Smart case ignores case unless a pattern
// holds a capital.
func (g grepCall) caseFlag() string {
	if g.icase {
		return "i"
	}
	if g.smart {
		for _, p := range g.patterns {
			if strings.IndexFunc(p, unicode.IsUpper) >= 0 {
				return ""
			}
		}
		return "i"
	}
	return ""
}

// The jq programs. Each reads every input with `inputs`, so a file operand and
// a stdin redirect take the same path.
const (
	jqPath = `def pstr: map(if type == "number" then "[\(.)]" ` +
		`elif test("^[A-Za-z_][A-Za-z0-9_]*$") then ".\(.)" else "[\(tojson)]" end) ` +
		`| join("") | if startswith(".") then . else "." + . end; `
	jqRecords = `foreach inputs as $r ({}; .[input_filename // "-"] += 1; ` +
		`{file: (input_filename // "-"), n: .[input_filename // "-"], r: $r}) ` +
		`| select(.r | any(.. | (objects | keys_unsorted[]), (scalars | tostring); hit)%s)`
	jqLeaves = `inputs as $doc | (input_filename // "-") as $file | $doc ` +
		`| path(.. | scalars) as $p | getpath($p) as $v ` +
		`| select(any(($p[] | strings), ($v | tostring); hit)%s)`
)

// jqProgram is the filter for files of one kind.
func (g grepCall) jqProgram(kind jsonKind, multi bool) string {
	not := ""
	if g.invert {
		not = " | not"
	}
	defs := fmt.Sprintf(`def hit: test($re; %q); `, g.caseFlag())
	showName := g.names == 1 || (g.names == 0 && multi)
	var body, line string
	if kind == jsonLines {
		body = fmt.Sprintf(jqRecords, not)
		if showName {
			line += `\(.file):`
		}
		if g.numbers {
			line += `\(.n):`
		}
		line += `\(.r | tojson)`
	} else {
		defs = jqPath + defs
		body = fmt.Sprintf(jqLeaves, not)
		if showName {
			line = `\($file):`
		}
		line += `\($p | pstr) = \($v | tojson)`
	}
	if g.count {
		return defs + "[" + body + "] | length"
	}
	return defs + body + ` | "` + line + `"`
}

// shellWord quotes s for the printer. A single quote cannot sit inside single
// quotes, so it closes the quote and adds an escaped quote.
func shellWord(s string) *syntax.Word {
	w := &syntax.Word{}
	for i, part := range strings.Split(s, "'") {
		if i > 0 {
			w.Parts = append(w.Parts, &syntax.Lit{Value: `\'`})
		}
		if part != "" {
			w.Parts = append(w.Parts, &syntax.SglQuoted{Value: part})
		}
	}
	if len(w.Parts) == 0 {
		w.Parts = []syntax.WordPart{&syntax.SglQuoted{}}
	}
	return w
}

// grepJSON holds the sniff results of a single Transform, because the
// fixed-point loop asks about the same file on each pass.
type grepJSON struct {
	dir   string
	cache map[string]jsonKind
}

func (j *grepJSON) kind(name string) jsonKind {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(j.dir, path)
	}
	if k, ok := j.cache[path]; ok {
		return k
	}
	k := sniffJSON(path)
	j.cache[path] = k
	return k
}

// operandKind is the kind all the inputs share. A glob expands as the shell
// expands it. A word the shell expands in another way yields notJSON.
func (j *grepJSON) operandKind(words []*syntax.Word) (jsonKind, int) {
	kind, count := notJSON, 0
	for _, w := range words {
		s, ok := literal(w)
		if !ok || s == "" || s == "-" || strings.HasPrefix(s, "~") || strings.ContainsAny(s, "{}") {
			return notJSON, 0
		}
		names := []string{s}
		if strings.ContainsAny(s, "*?[") {
			pattern := s
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(j.dir, pattern)
			}
			names, _ = filepath.Glob(pattern)
			if len(names) == 0 {
				return notJSON, 0
			}
		}
		for _, n := range names {
			k := j.kind(n)
			if k == notJSON {
				return notJSON, 0
			}
			if kind, ok = merge(kind, k); !ok {
				return notJSON, 0
			}
			count++
		}
	}
	return kind, count
}

// stdinFile is the path a statement redirects to stdin.
func stdinFile(s *syntax.Stmt) (*syntax.Word, bool) {
	var found *syntax.Word
	for _, r := range s.Redirs {
		if r.Op == syntax.RdrIn && (r.N == nil || r.N.Value == "0") {
			found = r.Word
		}
	}
	return found, found != nil
}

func (j *grepJSON) rewrite(s *syntax.Stmt) {
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok {
		return
	}
	e, ok := effectiveCommand(c)
	if !ok {
		return
	}
	spec, ok := grepPrograms[e.name]
	if !ok || wrappedByXargs(c.Args[:e.index]) {
		return
	}
	g, ok := parseGrep(spec, c.Args[e.index+1:])
	if !ok {
		return
	}
	inputs := g.files
	if len(inputs) == 0 {
		in, ok := stdinFile(s)
		if !ok {
			return
		}
		inputs = []*syntax.Word{in}
	}
	kind, count := j.operandKind(inputs)
	if kind == notJSON || (g.count && count > 1) {
		return
	}
	re, ok := g.onigRegex()
	if !ok {
		return
	}
	args := append([]*syntax.Word{}, c.Args[:e.index]...)
	args = append(args, word("jq"), word("-n"), word("-r"), word("--arg"), word("re"), shellWord(re),
		shellWord(g.jqProgram(kind, count > 1)))
	c.Args = append(args, g.files...)
}

func wrappedByXargs(prefix []*syntax.Word) bool {
	for _, w := range prefix {
		if s, ok := literal(w); ok && filepath.Base(s) == "xargs" {
			return true
		}
	}
	return false
}

func (j *grepJSON) apply(f *syntax.File) {
	syntax.Walk(f, func(n syntax.Node) bool {
		if s, ok := n.(*syntax.Stmt); ok {
			j.rewrite(s)
		}
		return true
	})
}
