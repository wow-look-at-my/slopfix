package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/treecomments"
)

// isDirectiveLine reports a line a tool reads rather than a reader. The C family
// spells it with no space after the marker, and the hash family carries the
// interpreter line and the linter pragma.
func isDirectiveLine(line string) bool {
	if treecomments.IsDirective(line) || isLintPragma(line) {
		return true
	}
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//", "#"} {
		rest, found := strings.CutPrefix(t, marker)
		if !found {
			continue
		}
		if marker == "#" && strings.HasPrefix(rest, "!") {
			return true // an interpreter line
		}
		name, _, hasColon := strings.Cut(rest, ":")
		if !hasColon || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		// `//go:build` and `# shellcheck:` carry no space before the colon.
		return true
	}
	return false
}

// lintPragmas open a comment that a linter or a type checker reads, after the marker and a space. A cut that drops one turns a suppressed warning back on.
var lintPragmas = []string{"eslint-disable", "eslint-enable", "@ts-ignore", "@ts-expect-error", "@ts-nocheck", "prettier-ignore", "shellcheck ", "istanbul ignore", "c8 ignore"}

// isLintPragma reports a line comment that a linter reads.
func isLintPragma(line string) bool {
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//", "#"} {
		rest, found := strings.CutPrefix(t, marker)
		if !found {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		for _, pragma := range lintPragmas {
			if strings.HasPrefix(rest, pragma) {
				return true
			}
		}
	}
	return false
}
