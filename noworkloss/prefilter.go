package noworkloss

import "strings"

// mayDestroy is the cheap gate in front of everything expensive: it matches raw
// substrings, and a false positive costs only a parse.
func mayDestroy(command string) bool {
	for _, n := range prefilterNeedles {
		if strings.Contains(command, n) {
			return true
		}
	}
	return false
}

// A script's own text is invisible to a raw scan, so the spellings that START
// a script parse too.
var prefilterNeedles = []string{
	"git", "rm", "mv", ">", "tee", "truncate",
	"bash", "sh ", "zsh", "source", ".sh",
}

// destructiveKeyword reports whether raw text names something that can destroy
// work. Only consulted when the parser failed or the analysis panicked.
func destructiveKeyword(command string) (string, bool) {
	for _, m := range destructiveMarkers {
		if strings.Contains(command, m.needle) {
			return m.label, true
		}
	}
	return "", false
}

var destructiveMarkers = []struct{ needle, label string }{
	{"--hard", "git reset --hard"},
	{"--force", "a --force flag"},
	{"force-with-lease", "git push --force-with-lease"},
	{"checkout", "git checkout"},
	{"cherry-pick", "git cherry-pick"},
	{"truncate", "truncate"},
	{"restore", "git restore"},
	{"rebase", "git rebase"},
	{"switch", "git switch"},
	{"reset", "git reset"},
	{"stash", "git stash"},
	{"clean", "git clean"},
	{"merge", "git merge"},
	{"pull", "git pull"},
	{"rm", "rm"},
	{"mv", "mv"},
}
