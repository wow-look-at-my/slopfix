package bashclean

import (
	"mvdan.cc/sh/v3/syntax"
	"regexp"
	"strings"
)

var (
	magicPath = regexp.MustCompile(`^/(proc|sys|dev)(/|$)`)
)

var (
	sizeZeroFlag = regexp.MustCompile(`^(-s|--size=)0+$`)
	allZeros     = regexp.MustCompile(`^0+$`)
	droppableRM  = regexp.MustCompile(`^--(recursive|force|verbose|interactive)$|^-[rRfvIi]+$`)
)

// isTruncateZero reports a truncate that empties a file, in every spelling of
// the size flag: separate, attached, or long with an equals sign.
func isTruncateZero(args []*syntax.Word) bool {
	for i, w := range args {
		s, ok := literal(w)
		if !ok {
			continue
		}
		if sizeZeroFlag.MatchString(s) {
			return true
		}
		if (s == "-s" || s == "--size") && i+1 < len(args) {
			if n, ok := literal(args[i+1]); ok && allZeros.MatchString(n) {
				return true
			}
		}
	}
	return false
}

// truncateTargets returns the files an emptying truncate would clear. Any
// OTHER flag fails, which sends the call to the deny: `-r RFILE` names a
// reference file, so keeping its value would recycle an untouched file.
func truncateTargets(args []*syntax.Word) ([]*syntax.Word, bool) {
	ops := []*syntax.Word{}
	skip, done := false, false
	for _, w := range args {
		if done {
			ops = append(ops, w)
			continue
		}
		if skip {
			skip = false
			continue
		}
		s, ok := literal(w)
		switch {
		case !ok:
			ops = append(ops, w)
		case s == "--":
			done = true
		case sizeZeroFlag.MatchString(s):
		case s == "-s" || s == "--size":
			skip = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
			return nil, false
		default:
			ops = append(ops, w)
		}
	}
	if len(ops) == 0 {
		return nil, false
	}
	if needsSeparator(ops) {
		return append([]*syntax.Word{word("--")}, ops...), true
	}
	return ops, true
}

func hasBadTruncate(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok || e.name != "truncate" || !isTruncateZero(c.Args[e.index+1:]) {
			return false
		}
		_, fine := truncateTargets(c.Args[e.index+1:])
		return !fine
	})
}

// rmUnknownFlags collects the flags this rule refuses to translate. `--` ends
// flag parsing, so everything after it is a path however dash-leading, and a
// lone `-` is a filename.
func rmUnknownFlags(args []*syntax.Word) bool {
	for _, w := range args {
		s, ok := literal(w)
		if !ok {
			continue
		}
		if s == "--" {
			return false
		}
		if strings.HasPrefix(s, "-") && len(s) > 1 && !droppableRM.MatchString(s) {
			return true
		}
	}
	return false
}

// hasBadRM scans the whole tree, the same reach as the rewrite's walk, so an
// `rm` inside `$( )` is covered too.
func hasBadRM(f *syntax.File) bool {
	return anyCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		return ok && e.name == "rm" && rmUnknownFlags(c.Args[e.index+1:])
	})
}
