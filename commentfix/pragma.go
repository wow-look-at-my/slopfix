package commentfix

import "strings"

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
