// grepjson.go rewrites a grep over JSON files into a jq query.
//
// grep reads a JSON file as lines of text. It matches escape sequences and
// punctuation, and it prints a fragment with no path to the value. jq tests the
// keys and the decoded values instead:
//
//	JSON Lines: each matching record, whole and compact, as grep prints a line
//	document:   each matching leaf as `.path.to[0].key = value`
//
// A grep with a flag outside the table below, a pattern that is not literal, or
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

type grepDialect int

const (
	dialectGrep grepDialect = iota
	dialectRg
)

type regexSyntax int

const (
	reBasic regexSyntax = iota
	reExtended
	rePerl
	reFixed
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

// A long flag maps to its short letter, so one switch handles both.
var grepLong = map[string]byte{
	"--ignore-case": 'i', "--invert-match": 'v', "--extended-regexp": 'E',
	"--fixed-strings": 'F', "--basic-regexp": 'G', "--perl-regexp": 'P',
	"--word-regexp": 'w', "--line-regexp": 'x', "--no-filename": 'h',
	"--with-filename": 'H', "--line-number": 'n', "--no-messages": 's',
	"--count": 'c',
}

var rgLong = map[string]byte{
	"--ignore-case": 'i', "--invert-match": 'v', "--fixed-strings": 'F',
	"--pcre2": 'P', "--word-regexp": 'w', "--line-regexp": 'x',
	"--no-filename": 'I', "--with-filename": 'H', "--line-number": 'n',
	"--no-line-number": 'N', "--count": 'c', "--case-sensitive": 's',
	"--smart-case": 'S', "--no-messages": 0,
}

// applyShort sets the flag a letter names. False means the letter is outside
// the table, and the rewrite stops.
func (g *grepCall) applyShort(d grepDialect, c byte) bool {
	switch c {
	case 0:
	case 'i':
		g.icase, g.smart = true, false
	case 'v':
		g.invert = true
	case 'F':
		g.syntax = reFixed
	case 'P':
		g.syntax = rePerl
	case 'w':
		g.word = true
	case 'x':
		g.whole = true
	case 'H':
		g.names = 1
	case 'n':
		g.numbers = true
	case 'c':
		g.count = true
	default:
		return g.applyDialect(d, c)
	}
	return true
}

func (g *grepCall) applyDialect(d grepDialect, c byte) bool {
	if d == dialectRg {
		switch c {
		case 'I':
			g.names = -1
		case 'N':
			g.numbers = false
		case 's':
			g.icase, g.smart = false, false
		case 'S':
			g.icase, g.smart = false, true
		default:
			return false
		}
		return true
	}
	switch c {
	case 'E':
		g.syntax = reExtended
	case 'G':
		g.syntax = reBasic
	case 'y':
		g.icase = true
	case 'h':
		g.names = -1
	case 's':
	default:
		return false
	}
	return true
}

func isColorFlag(s string) bool {
	return s == "--color" || s == "--colour" || strings.HasPrefix(s, "--color=") || strings.HasPrefix(s, "--colour=")
}

// parseGrep reads the arguments after the program word. Every word it keeps
// must be literal, because the pattern becomes part of the jq regex.
func parseGrep(d grepDialect, start regexSyntax, args []*syntax.Word) (grepCall, bool) {
	g := grepCall{syntax: start}
	long := grepLong
	if d == dialectRg {
		long = rgLong
	}
	var positional []*syntax.Word
	explicit, done := false, false
	for i := 0; i < len(args); i++ {
		w := args[i]
		s, ok := literal(w)
		if done || !ok || s == "-" || !strings.HasPrefix(s, "-") {
			positional = append(positional, w)
			continue
		}
		switch {
		case s == "--":
			done = true
		case s == "-e" || s == "--regexp":
			if i+1 >= len(args) {
				return g, false
			}
			p, ok := literal(args[i+1])
			if !ok {
				return g, false
			}
			g.patterns = append(g.patterns, p)
			explicit = true
			i++
		case strings.HasPrefix(s, "--regexp="):
			g.patterns = append(g.patterns, strings.TrimPrefix(s, "--regexp="))
			explicit = true
		case isColorFlag(s):
		case strings.HasPrefix(s, "--"):
			c, known := long[s]
			if !known || !g.applyShort(d, c) {
				return g, false
			}
		default:
			if !g.shortCluster(d, s[1:], args, &i, &explicit) {
				return g, false
			}
		}
	}
	if !explicit {
		if len(positional) == 0 {
			return g, false
		}
		p, ok := literal(positional[0])
		if !ok {
			return g, false
		}
		g.patterns = []string{p}
		positional = positional[1:]
	}
	g.files = positional
	return g, true
}

// shortCluster reads a cluster such as `-inE`. An `e` takes the rest of the
// cluster as its pattern, or the next word when the rest is empty.
func (g *grepCall) shortCluster(d grepDialect, cluster string, args []*syntax.Word, i *int, explicit *bool) bool {
	for j := 0; j < len(cluster); j++ {
		if cluster[j] != 'e' {
			if !g.applyShort(d, cluster[j]) {
				return false
			}
			continue
		}
		p := cluster[j+1:]
		if p == "" {
			if *i+1 >= len(args) {
				return false
			}
			v, ok := literal(args[*i+1])
			if !ok {
				return false
			}
			p = v
			*i++
		}
		g.patterns = append(g.patterns, p)
		*explicit = true
		return true
	}
	return true
}

var reMeta = regexp.MustCompile(`[\\^$.|?*+()\[\]{}]`)

// onigRegex builds the regex jq's `test` reads. jq links Oniguruma with its
// Perl syntax, so a POSIX pattern needs translation.
func (g grepCall) onigRegex() (string, bool) {
	var alts []string
	for _, p := range g.patterns {
		for _, line := range strings.Split(p, "\n") {
			var re string
			switch g.syntax {
			case reFixed:
				re = reMeta.ReplaceAllString(line, `\$0`)
			case rePerl:
				re = line
			default:
				var ok bool
				if re, ok = posixToOnig(line, g.syntax == reBasic); !ok {
					return "", false
				}
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

// posixToOnig rewrites a POSIX basic or extended pattern for Oniguruma. In a
// basic pattern, a bare `( ) { } | + ?` is literal and the escaped form is the
// operator. Oniguruma reads them the other way around.
func posixToOnig(p string, basic bool) (string, bool) {
	var b strings.Builder
	atStart := true
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '[':
			n, ok := bracket(p[i:], &b)
			if !ok {
				return "", false
			}
			i += n - 1
			atStart = false
			continue
		case c == '\\':
			if i+1 >= len(p) {
				return "", false
			}
			i++
			e := p[i]
			switch {
			case e == '<' || e == '>':
				b.WriteString(`\b`)
			case basic && strings.IndexByte("(){}|+?", e) >= 0:
				b.WriteByte(e)
				atStart = e == '(' || e == '|'
				continue
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
		case basic && strings.IndexByte("(){}|+?", c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '*' && atStart:
			b.WriteString(`\*`)
		case basic && c == '^' && !atStart:
			b.WriteString(`\^`)
		case basic && c == '$' && !endsGroup(p[i+1:]):
			b.WriteString(`\$`)
		default:
			b.WriteByte(c)
			if (!basic && (c == '(' || c == '|')) || (c == '^' && atStart) {
				atStart = true
				continue
			}
		}
		atStart = false
	}
	return b.String(), true
}

// endsGroup asks whether a `$` sits where a basic pattern reads it as an
// anchor: at the end, or before `\)` or `\|`.
func endsGroup(rest string) bool {
	return rest == "" || strings.HasPrefix(rest, `\)`) || strings.HasPrefix(rest, `\|`)
}

// bracket copies a POSIX bracket expression and returns its length. A
// backslash, a nested `[` and `&` are literal in POSIX and special in
// Oniguruma, so each gets a backslash.
func bracket(p string, b *strings.Builder) (int, bool) {
	b.WriteByte('[')
	i := 1
	if i < len(p) && p[i] == '^' {
		b.WriteByte('^')
		i++
	}
	if i < len(p) && p[i] == ']' {
		b.WriteString(`\]`)
		i++
	}
	for i < len(p) {
		c := p[i]
		switch {
		case c == ']':
			b.WriteByte(']')
			return i + 1, true
		case c == '[' && i+1 < len(p) && strings.IndexByte(":.=", p[i+1]) >= 0:
			end := strings.Index(p[i+2:], string(p[i+1])+"]")
			if end < 0 {
				return 0, false
			}
			b.WriteString(p[i : i+2+end+2])
			i += 2 + end + 2
			continue
		case c == '\\' || c == '[' || c == '&':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	return 0, false
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
			if k == notJSON || (kind != notJSON && k != kind) {
				return notJSON, 0
			}
			kind = k
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

func grepProgram(name string) (grepDialect, regexSyntax, bool) {
	switch name {
	case "grep":
		return dialectGrep, reBasic, true
	case "egrep":
		return dialectGrep, reExtended, true
	case "fgrep":
		return dialectGrep, reFixed, true
	case "rg":
		return dialectRg, rePerl, true
	}
	return 0, 0, false
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
	d, start, ok := grepProgram(e.name)
	if !ok || wrappedByXargs(c.Args[:e.index]) {
		return
	}
	g, ok := parseGrep(d, start, c.Args[e.index+1:])
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
