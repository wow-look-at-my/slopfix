// A word carries its literal text plus whether that text is the whole story.

package noworkloss

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"mvdan.cc/sh/v3/syntax"
)

// A word carries its literal text plus whether that text is the whole story.
// A write aimed at an unknowable path must deny.
type word struct {
	text   string
	static bool
}

type redirTarget struct {
	op syntax.RedirOperator
	// fd is the descriptor the redirect rebinds, as written. Empty means stdout.
	fd   string
	file word
}

// touchesStdout reports whether a redirect points stdout at its target.
func touchesStdout(r redirTarget) bool {
	if r.op == syntax.RdrAll || r.op == syntax.AppAll {
		return true
	}
	return r.fd == "" || r.fd == "1"
}

// redirLabel names a redirect the way the reader wrote it, so a message about
// a stderr redirect never quotes it back as a stdout redirect.
func redirLabel(r redirTarget, op string) string {
	return r.fd + op + " " + r.file.text
}

// An executable unit: an argv with the directory it runs in, plus the redirects
// attached to it.
type segment struct {
	argv   []word
	cwd    string
	redirs []redirTarget
	// relocated marks a GIT_DIR / GIT_WORK_TREE prefix, which moves the repository away from the path words.
	relocated bool
	// stdinScript marks a stage fed by a pipe or a heredoc, where an interpreter runs an unresolvable script.
	stdinScript bool
	// fromScript marks a unit read out of a script FILE, whose writes belong.
	fromScript bool
}

var repoRelocatingEnv = set.Of[string]("GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE")

const (
	maxWalkDepth   = 32
	maxSegments    = 4096
	maxScriptBytes = 1 << 20
	maxScriptDepth = 3
	unknownDirText = ""
)

// parseSegments flattens a command into every unit that will execute, in
// execution order, tracking the working directory across the sequence. It
// reports the blockers it hit -- a script it could not analyse -- separately
// from the segments, because those deny on their own.
func parseSegments(command, cwd string) (segs []segment, blockers []blocker, ok bool) {
	f, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, nil, false
	}
	unsafe, multi, abort := unsafeVarNames(f.Stmts)
	w := &walker{vars: varTable{}, unsafeVars: unsafe, multiVars: multi, varsDisabled: abort, scopeOK: true}
	base := cwd
	w.stmts(f.Stmts, &base)
	return w.segs, w.blockers, true
}

type walker struct {
	segs        []segment
	blockers    []blocker
	depth       int
	scriptDepth int
	// fileDepth counts how deep the walk stands inside a script FILE.
	fileDepth int
	piped     bool
	// vars holds every variable proven to hold a static value. unsafeVars
	vars         varTable
	unsafeVars   map[string]bool
	multiVars    map[string]bool
	varsDisabled bool
	// scopeOK marks a scope whose whole program text this walk has read.
	scopeOK bool
}

// resolve renders a word, substituting a variable this hook has proven safe.
func (w *walker) resolve(wd *syntax.Word) word {
	if w.scopeOK && !w.varsDisabled && len(w.vars) > 0 {
		return resolveWord(wd, w.vars)
	}
	return wordText(wd)
}

// enterScope swaps in the variable scope of a script the walk is about to
// follow as a fresh shell. It binds $0 to the file the script came from. It
// returns the function that restores the caller's scope.
func (w *walker) enterScope(stmts []*syntax.Stmt, self string) func() {
	prev, prevUnsafe, prevMulti := w.vars, w.unsafeVars, w.multiVars
	prevDisabled, prevOK := w.varsDisabled, w.scopeOK
	unsafe, multi, abort := unsafeVarNames(stmts)
	w.vars, w.unsafeVars, w.multiVars = varTable{}, unsafe, multi
	w.varsDisabled, w.scopeOK = abort, true
	if self != "" {
		w.vars["0"] = word{text: self, static: true}
	}
	return func() {
		w.vars, w.unsafeVars, w.multiVars = prev, prevUnsafe, prevMulti
		w.varsDisabled, w.scopeOK = prevDisabled, prevOK
	}
}

// A blocker is a piece of the command whose text this walk could not read.
// fromScript records whether it was found inside a script FILE.
type blocker struct {
	text       string
	fromScript bool
}

// block records a blocker, stamping where it was found, so a site added later cannot forget.
func (w *walker) block(text string) {
	w.blockers = append(w.blockers, blocker{text: text, fromScript: w.fileDepth > 0})
}

func (w *walker) full() bool { return len(w.segs) >= maxSegments }

func (w *walker) stmts(sts []*syntax.Stmt, cwd *string) {
	for _, st := range sts {
		w.stmt(st, cwd)
	}
}

func (w *walker) stmt(st *syntax.Stmt, cwd *string) {
	if st == nil || w.full() {
		return
	}
	var rs []redirTarget
	stdin := w.piped
	for _, r := range st.Redirs {
		if r == nil || r.Word == nil {
			continue
		}
		w.scanSubst(r.Word, *cwd)
		if r.Op == syntax.Hdoc || r.Op == syntax.DashHdoc || r.Op == syntax.WordHdoc {
			stdin = true
			continue // a heredoc is input; its "target" is the delimiter word
		}
		// A `< file` redirect is the same script-on-stdin as a pipe. The
		// interpreter rule asks separately whether a script was already named.
		if r.Op == syntax.RdrIn {
			stdin = true
			continue // input, not a target this command writes
		}
		fd := ""
		if r.N != nil {
			fd = r.N.Value
		}
		rs = append(rs, redirTarget{op: r.Op, fd: fd, file: w.resolve(r.Word)})
	}
	w.command(st.Cmd, cwd, rs, stdin)
}

// command dispatches on node type. The cwd pointer is shared only where the
// shell itself shares it. A cd carries forward across `&&`, `||` and `;`,
// while a pipe stage, a subshell and a conditional body each get a copy.
func (w *walker) command(c syntax.Command, cwd *string, rs []redirTarget, stdin bool) {
	if c == nil || w.full() {
		return
	}
	w.depth++
	defer func() { w.depth-- }()
	if w.depth > maxWalkDepth {
		w.block("the command nests deeper than this hook will follow")
		return
	}

	switch x := c.(type) {
	case *syntax.CallExpr:
		w.call(x, cwd, rs, stdin)
	case *syntax.BinaryCmd:
		if x.Op == syntax.Pipe || x.Op == syntax.PipeAll {
			left, right := *cwd, *cwd
			was := w.piped
			w.piped = stdin
			w.stmt(x.X, &left)
			w.piped = true
			w.stmt(x.Y, &right)
			w.piped = was
			return
		}
		w.stmt(x.X, cwd)
		w.stmt(x.Y, cwd)
	case *syntax.Subshell:
		w.bare(rs, *cwd)
		w.isolated(x.Stmts, *cwd)
	case *syntax.Block:
		w.bare(rs, *cwd)
		w.stmts(x.Stmts, cwd)
	case *syntax.IfClause:
		w.bare(rs, *cwd)
		w.isolated(x.Cond, *cwd)
		w.isolated(x.Then, *cwd)
		if x.Else != nil {
			w.command(x.Else, cwd, nil, false)
		}
	case *syntax.WhileClause:
		w.bare(rs, *cwd)
		w.isolated(x.Cond, *cwd)
		w.isolated(x.Do, *cwd)
	case *syntax.ForClause:
		w.bare(rs, *cwd)
		w.isolated(x.Do, *cwd)
	case *syntax.CaseClause:
		w.bare(rs, *cwd)
		for _, item := range x.Items {
			if item != nil {
				w.isolated(item.Stmts, *cwd)
			}
		}
	case *syntax.FuncDecl:
		// A function body executes when the function is called, and this hook
		// cannot know whether that happens in this command or a later call. Its
		if x.Body != nil {
			local := *cwd
			w.stmt(x.Body, &local)
		}
	case *syntax.TimeClause:
		if x.Stmt != nil {
			w.stmt(x.Stmt, cwd)
		}
	case *syntax.CoprocClause:
		if x.Stmt != nil {
			local := *cwd
			w.stmt(x.Stmt, &local)
		}
	default:
		w.bare(rs, *cwd)
	}
}

// bare records a redirect that hangs off a compound rather than an argv:
// `{ ...; } > f` and `(...) > f` truncate f with no command word of their own.
func (w *walker) bare(rs []redirTarget, cwd string) {
	if len(rs) > 0 {
		w.segs = append(w.segs, segment{cwd: cwd, redirs: rs, fromScript: w.fileDepth > 0})
	}
}

func (w *walker) isolated(sts []*syntax.Stmt, cwd string) {
	local := cwd
	w.stmts(sts, &local)
}

func (w *walker) call(c *syntax.CallExpr, cwd *string, rs []redirTarget, stdin bool) {
	relocated := false
	// A pure assignment statement -- no command word of its own -- persists
	pureAssign := len(c.Args) == 0
	for _, a := range c.Assigns {
		if a == nil {
			continue
		}
		if a.Name != nil && repoRelocatingEnv.Contains(a.Name.Value) {
			relocated = true
		}
		if a.Value != nil {
			w.scanSubst(a.Value, *cwd)
		}
		if pureAssign && w.scopeOK && !w.varsDisabled && a.Name != nil && a.Value != nil {
			switch {
			case w.multiVars[a.Name.Value]:
				// Several assignments, so no reading can be pinned at all.
			case !w.unsafeVars[a.Name.Value]:
				if v := w.resolve(a.Value); v.static {
					w.vars[a.Name.Value] = v
				} else if v, ok := mktempPath(a.Value); ok {
					w.vars[a.Name.Value] = v
				}
			default:
				// A lone assignment that might not run: the value is unknown,
				// but a mktemp path's directory is the same either way.
				if v, ok := mktempPath(a.Value); ok {
					w.vars[a.Name.Value] = v
				}
			}
		}
	}
	argv := make([]word, 0, len(c.Args))
	for _, wd := range c.Args {
		w.scanSubst(wd, *cwd)
		argv = append(argv, w.resolve(wd))
	}
	if len(argv) == 0 {
		w.bare(rs, *cwd)
		return
	}
	// An `env VAR=VAL` prefix carries the same relocation as a bare prefix.
	for _, a := range argv {
		if i := strings.Index(a.text, "="); i > 0 && repoRelocatingEnv.Contains(a.text[:i]) {
			relocated = true
		}
	}
	eff := stripWrappers(argv)
	if len(eff) == 0 {
		w.bare(rs, *cwd)
		return
	}
	name := commandName(eff[0].text)
	if name == "cd" || name == "pushd" {
		*cwd = resolveDir(*cwd, eff)
		return
	}
	if w.expand(name, eff, *cwd) {
		w.bare(rs, *cwd)
		return
	}
	w.segs = append(w.segs, segment{
		argv: eff, cwd: *cwd, redirs: rs,
		relocated: relocated, stdinScript: stdin,
		fromScript: w.fileDepth > 0,
	})
}
