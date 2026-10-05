package bashclean

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// noRevert turns every `git revert` call into the no-op `:`. A revert undoes
// work wholesale, and the owner rules it out for every session.
func noRevert(c *syntax.CallExpr) {
	if isGitRevert(c) {
		c.Args = []*syntax.Word{word(":")}
	}
}

// isGitRevert reads the subcommand as parseGitRM does: the leading word after
// `git` that is not a flag, past the value of `-C` and `-c`.
func isGitRevert(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "git" {
		return false
	}
	args := c.Args[e.index+1:]
	for i := 0; i < len(args); i++ {
		s, static := literal(args[i])
		if !static {
			return false
		}
		if s == "-C" || s == "-c" {
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			continue
		}
		return s == "revert"
	}
	return false
}
