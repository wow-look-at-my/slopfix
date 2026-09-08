package shellwalk

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// StdinMarkers are the operands naming stdin as the source of the program.
var StdinMarkers = set.Of[string]("-", "/dev/stdin")

// NamesAScript reports whether an interpreter invocation already carries a file
// for the interpreter to RUN. Any operand counts, static or not: stdin is then
// the program's INPUT rather than its program.
func NamesAScript(args []Word) bool {
	dashDash := false
	for _, a := range args {
		t := a.Text
		switch {
		case dashDash:
			return !StdinMarkers.Contains(t)
		case t == "--":
			dashDash = true
		case StdinMarkers.Contains(t):
			// A stdin marker is an operand, but it names stdin, not a script.
		case strings.HasPrefix(t, "-") && len(t) > 1:
			// a flag
		case t == "":
			// an expansion that resolved to nothing names no script
		default:
			return true
		}
	}
	return false
}

// ShellNoExec reports whether a shell was told to parse without executing: -n,
// --noexec, or an n inside a single-dash cluster. Nothing is written, so
// following the script's text is wrong. An operand ends the flags.
func ShellNoExec(argv []Word) bool {
	for i := 1; i < len(argv); i++ {
		t := argv[i].Text
		if t == "--noexec" {
			return true
		}
		if t == "-c" || t == "--" || !strings.HasPrefix(t, "-") {
			return false
		}
		if !strings.HasPrefix(t, "--") && strings.ContainsRune(t[1:], 'n') {
			return true
		}
	}
	return false
}

// IsShell reports whether a resolved program name is a shell whose -c string or
// script file is more shell source.
func IsShell(name string) bool {
	switch name {
	case "sh", "bash", "dash", "zsh", "ksh", "mksh", "ash", "busybox":
		return true
	}
	return false
}
