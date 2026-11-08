// segment_scripts.go follows a command into the shell, script or exec it runs.

package noworkloss

import (
	"os"
	"strings"

	"github.com/wow-look-at-my/slopfix/shellwalk"
	"mvdan.cc/sh/v3/syntax"
)

// expand follows the indirections whose text this hook can read, and reports
// true when the walk consumed the call.
func (w *walker) expand(name string, eff []word, cwd string) bool {
	switch {
	case name == "alias":
		// `alias w='sed -i'` hides a writer behind a name the walk would
		// otherwise read as an unknown program.
		for _, a := range eff[1:] {
			if i := strings.Index(a.text, "="); i > 0 {
				w.script(a.text[i+1:], cwd, "an alias definition", "")
			}
		}
		return true
	case isShell(name):
		return w.shellCall(eff, cwd)
	case name == "source" || name == ".":
		if len(eff) > 1 {
			w.scriptFile(eff[1], cwd, false)
		}
		return true
	case name == "find":
		w.findExec(eff, cwd)
		return false
	}
	// `./deploy.sh` names a file rather than a program on PATH. When its shebang
	// says shell, its text is readable and gets the same treatment.
	if strings.Contains(eff[0].text, "/") && eff[0].static && hasShellShebang(abs(cwd, eff[0].text)) {
		w.scriptFile(eff[0], cwd, true)
		return true
	}
	return false
}

func (w *walker) shellCall(eff []word, cwd string) bool {
	// `bash -n script.sh` parses and never runs, so nothing it names is written.
	if shellNoExec(eff) {
		return true
	}
	for i := 1; i < len(eff); i++ {
		t := eff[i].text
		if t == "-c" {
			if i+1 >= len(eff) {
				return true
			}
			if !eff[i+1].static {
				w.block("a shell -c script assembled from an expansion, whose writes cannot be resolved")
				return true
			}
			w.script(eff[i+1].text, cwd, "a shell -c script", "")
			return true
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		if eff[i].static && isAPE(abs(cwd, t)) {
			w.emit(eff[i:], cwd)
			return true
		}
		w.scriptFile(eff[i], cwd, true)
		return true
	}
	// A bare `bash` reads its script from stdin, which is not in the text.
	w.block("a shell reading its script from stdin, whose writes cannot be resolved")
	return true
}

// shellNoExec reports whether the shell was told to parse without executing, in
// which case nothing the script names is written.
func shellNoExec(eff []word) bool {
	shared := make([]shellwalk.Word, len(eff))
	for i, a := range eff {
		shared[i] = shellwalk.Word{Text: a.text, Static: a.static}
	}
	return shellwalk.ShellNoExec(shared)
}

// script parses shell source found inside the command and folds its segments
// into the same walk. So a deeply nested write gets the same verdict as a
// write at top level. self names the file a fresh shell was started from, and is empty for
// text that runs in the caller's own scope.
func (w *walker) script(src, cwd, what, self string) {
	if w.scriptDepth >= maxScriptDepth {
		w.block(what + ", nested deeper than this hook will follow")
		return
	}
	f, err := syntax.NewParser().Parse(strings.NewReader(src), "")
	if err != nil {
		w.block(what + ", which does not parse as shell")
		return
	}
	w.scriptDepth++
	if self != "" {
		// The depth rises only after the file's text is in hand: a blocker
		w.fileDepth++
		defer func() { w.fileDepth-- }()
		defer w.enterScope(f.Stmts, self)()
	} else {
		prev := w.scopeOK
		w.scopeOK = false
		defer func() { w.scopeOK = prev }()
	}
	w.isolated(f.Stmts, cwd)
	w.scriptDepth--
}

// scriptFile follows a shell script on disk. A script that does not exist writes
// nothing, so it is left alone. A script that exists and cannot be read or parsed is
// the write-elsewhere-then-run bypass and denies. fresh marks a script started as a
// new shell, whose variables are entirely its own text. A sourced file shares the
// caller's scope and passes false.
func (w *walker) scriptFile(f word, cwd string, fresh bool) {
	if !f.static {
		w.block("a script path built from an expansion, whose writes cannot be resolved")
		return
	}
	path := abs(cwd, f.text)
	if path == "" {
		w.block("a script at a path that is not statically known")
		return
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return
	}
	if st.Size() > maxScriptBytes {
		w.block("the script " + f.text + ", which is too large to analyse")
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		w.block("the script " + f.text + ", which cannot be read")
		return
	}
	self := ""
	if fresh {
		self = path
	}
	// Only a NEW shell counts as a program of its own.
	w.script(string(src), cwd, "the script "+f.text, self)
}

// findExec lifts the utility out of `find ... -exec <argv> ;` and walks it as a
// call of its own. Without this, `find. -name '*.go' -exec sed -i s/a/b/ {} +`
// reads as a single invocation of a program called find.
func (w *walker) findExec(eff []word, cwd string) {
	for i := 1; i < len(eff); i++ {
		t := eff[i].text
		if t != "-exec" && t != "-execdir" && t != "-ok" && t != "-okdir" {
			continue
		}
		var sub []word
		for j := i + 1; j < len(eff); j++ {
			if eff[j].text == ";" || eff[j].text == "+" {
				break
			}
			sub = append(sub, eff[j])
		}
		if len(sub) == 0 {
			continue
		}
		// -execdir runs in the directory of each match, which the text does not
		dir := cwd
		if t == "-execdir" || t == "-okdir" {
			dir = unknownDirText
		}
		w.emit(sub, dir)
	}
}

// emit runs a lifted argv through the same wrapper stripping and expansion the
// walker applies to a call it parsed itself.
func (w *walker) emit(argv []word, cwd string) {
	eff := stripWrappers(argv)
	if len(eff) == 0 {
		return
	}
	if w.expand(commandName(eff[0].text), eff, cwd) {
		return
	}
	w.segs = append(w.segs, segment{argv: eff, cwd: cwd, fromScript: w.fileDepth > 0})
}

// A command substitution runs its own shell, so its cd is contained. However,
// the command inside is every bit as able to write as a command at top level.
func (w *walker) scanSubst(wd *syntax.Word, cwd string) {
	if wd == nil || w.full() || w.depth > maxWalkDepth {
		return
	}
	for _, p := range wd.Parts {
		switch x := p.(type) {
		case *syntax.CmdSubst:
			w.depth++
			w.isolated(x.Stmts, cwd)
			w.depth--
		case *syntax.ProcSubst:
			w.depth++
			w.isolated(x.Stmts, cwd)
			w.depth--
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if sub, ok := dp.(*syntax.CmdSubst); ok {
					w.depth++
					w.isolated(sub.Stmts, cwd)
					w.depth--
				}
			}
		}
	}
}
