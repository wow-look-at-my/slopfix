package noworkloss

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/shellwalk"
	"os"
	"path/filepath"
	"strings"
)

// An interpreter handed a script writes whatever the script says, and the script
// is not something this hook can resolve. Distinct shapes follow: an inline
// script denies wherever it runs, because its targets are unknowable; a script
// FILE is followed when it is shell (see segment.go) and judged by where it
// lives when it is not.

// evalFlags names, per interpreter, the flags that hand it a program rather than
// a file. The set is per-tool because the spelling is: ruby's -E sets an
// encoding while perl's runs code.
var evalFlags = map[string][]string{
	"node":      {"-e", "--eval", "-p", "--print"},
	"nodejs":    {"-e", "--eval", "-p", "--print"},
	"bun":       {"-e", "--eval", "-p", "--print"},
	"deno":      {"--eval"},
	"ruby":      {"-e"},
	"perl":      {"-e", "-E"},
	"python":    {"-c"},
	"php":       {"-r"},
	"lua":       {"-e"},
	"luajit":    {"-e"},
	"tclsh":     {},
	"rscript":   {"-e"},
	"osascript": {"-e"},
	"ts-node":   {"-e"},
	"tsx":       {"-e"},
	"jq":        nil, // jq has no way to write a file; listed so nobody adds a flag
}

// Editors exist to rewrite the file they open, so they are writers by default
// rather than by flag.
var editors = set.Of[string]("ed", "ex", "vi", "vim", "nvim",
	"emacs", "emacsclient")

func interpreterWrites(seg segment, name string, rest []word, roots []string) ([]write, bool) {
	name = stripVersion(name)
	if editors.Contains(name) {
		return editorWrites(seg, name, rest), true
	}
	flags, ok := evalFlags[name]
	if !ok {
		return nil, false
	}
	if flags == nil {
		return nil, true // recognised and unable to write
	}
	for _, a := range rest {
		for _, f := range flags {
			if a.text == f || strings.HasPrefix(a.text, f+"=") {
				return []write{{route: name + " " + f, opaque: "an inline " + name + " script; the files it writes are named in code this hook cannot resolve"}}, true
			}
		}
		// `deno eval` and the like put the program behind a subcommand.
		if a.text == "eval" && name == "deno" {
			return []write{{route: "deno eval", opaque: "an inline deno script; the files it writes are named in code this hook cannot resolve"}}, true
		}
		if a.text == "-" {
			return []write{{route: name + " -", opaque: "a " + name + " script read from stdin, which is not in the command text"}}, true
		}
	}
	// A pipe or a `< file` redirect only carries a SCRIPT when the interpreter
	// was given nothing else to run: a named script means stdin is its input.
	if seg.stdinScript && !namesAScript(rest) {
		return []write{{route: name + " (stdin)", opaque: "a " + name + " script piped in on stdin, which is not in the command text"}}, true
	}
	return scratchScriptWrites(seg, name, rest, roots), true
}

// scratchScriptWrites closes the write-elsewhere-then-run route: the Write tool
// aimed at /tmp is allowed, so running that file is the half that puts its
// content into the tree. A script that lives inside the tree is not this -- it
// got there through Write or Edit and is visible in the diff -- and a system
// script is the tool it belongs to, so only a scratch script denies.
func scratchScriptWrites(seg segment, name string, rest []word, roots []string) []write {
	_, operands := scanArgs(rest, noFlags)
	for _, o := range operands {
		if !o.static {
			return []write{{route: name, opaque: "a " + name + " script path built from an expansion, so what it runs is not in the command text"}}
		}
		p := abs(seg.cwd, o.text)
		if _, guarded := insideGuarded(roots, p); guarded {
			break // a script in the tree got there through Write or Edit
		}
		if isSessionScratchpad(p) {
			break // the harness tells the session to put temp files here
		}
		if p != "" && isScratchPath(p) {
			return []write{{route: name + " " + o.text, opaque: "a " + name + " script under a temporary directory; write the file with Write or Edit instead of generating it from a scratch script"}}
		}
		break // the leading operand is the script; the rest are its arguments
	}
	return nil
}

// namesAScript reports whether the invocation already carries a file for the
// interpreter to run. A stdin marker names stdin rather than a script.
func namesAScript(rest []word) bool {
	shared := make([]shellwalk.Word, len(rest))
	for i, a := range rest {
		shared[i] = shellwalk.Word{Text: a.text, Static: a.static}
	}
	return shellwalk.NamesAScript(shared)
}

func editorWrites(seg segment, name string, rest []word) []write {
	// -s is silent/script mode for ed, ex and vim and takes no value; reading it
	// as a value flag swallows the file operand and the write disappears.
	valueFlags := set.Of[string]("-c", "--command", "--eval", "-u", "-i", "--load")
	flags, operands := scanArgs(rest, valueFlags)
	if len(operands) > 0 {
		return []write{{route: name, paths: operands, dir: seg.cwd}}
	}
	// No file operand and a script to run means the target is named inside the
	// script. No file and no script means the invocation edits nothing at all --
	// `emacs --version` is not an editing session.
	for _, f := range []string{"-c", "--command", "--eval", "--batch", "--script", "-s"} {
		if _, ok := flags[f]; ok {
			return []write{{route: name, opaque: "an " + name + " session whose target file is named in its script rather than on the command line"}}
		}
	}
	if seg.stdinScript {
		return []write{{route: name, opaque: "an " + name + " session driven from stdin, so its target file is not in the command text"}}
	}
	return nil
}

// isSessionScratchpad recognises the directory the harness ITSELF tells a
// session to use for temporary files, so denying the scripts written there
// would refuse the documented workflow. It needs a "scratchpad" segment under
// an ancestor named claude, so an ordinary /tmp/scratchpad does not qualify.
func isSessionScratchpad(p string) bool {
	if p == "" || !isScratchPath(p) {
		return false
	}
	segs := strings.Split(filepath.Clean(p), string(filepath.Separator))
	claude := false
	for _, s := range segs {
		if s == "claude" || strings.HasPrefix(s, "claude-") || strings.HasPrefix(s, "claude_") {
			claude = true
			continue
		}
		if claude && s == "scratchpad" {
			return true
		}
	}
	return false
}

// scratch directories: the places a file can appear without any review.
func isScratchPath(p string) bool {
	dirs := []string{"/tmp", "/var/tmp", "/dev/shm"}
	if t := os.Getenv("TMPDIR"); t != "" {
		dirs = append(dirs, filepath.Clean(t))
	}
	for _, d := range dirs {
		if p == d || strings.HasPrefix(p, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// stripVersion turns a versioned interpreter name into the tool, so a rule is
// written for the tool instead of for every installed version.
func stripVersion(name string) string {
	trimmed := strings.TrimRight(name, "0123456789.")
	if trimmed == "" {
		return strings.ToLower(name)
	}
	return strings.ToLower(trimmed)
}

// allowedFormatter names the tools that rewrite files by design. Every tool
// below writes only a canonical reformat, which is what separates them from
// `sed -i`. A tool absent from the list denies, and runs through a recipe.
func allowedFormatter(name string, rest []word) bool {
	sub := ""
	if len(rest) > 0 {
		sub = rest[0].text
	}
	switch stripVersion(name) {
	case "gofmt", "goimports", "shfmt", "rustfmt", "clang-format", "prettier", "dprint", "biome":
		return true
	case "go-toolchain":
		// The org's Go entry point: it tidies go.mod, formats source and runs the
		return true
	case "go":
		return sub == "generate" || sub == "fmt" || sub == "mod"
	case "cargo":
		return sub == "fmt"
	case "terraform":
		return sub == "fmt"
	}
	return false
}
