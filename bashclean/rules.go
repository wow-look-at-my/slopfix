package bashclean

import (
	"mvdan.cc/sh/v3/syntax"
	"regexp"
	"strings"
)

var (
	magicPath      = regexp.MustCompile(`^/(proc|sys|dev)(/|$)`)
	longValuedRead = regexp.MustCompile(`^--(lines|bytes|sleep-interval|pid|max-unchanged-stats)$`)
	shortValued    = regexp.MustCompile(`^-[A-Za-z]*[ncs]$`)
	oldStyleLimit  = regexp.MustCompile(`^\+[0-9]+$`)
	sedQuietFlag   = regexp.MustCompile(`^-[A-Za-z]*n[A-Za-z]*$`)
	sedScriptFlag  = regexp.MustCompile(`^-[A-Za-z]*[ef][A-Za-z]*$`)
)

// readOperands returns the file operands of a cat/head/tail call. Flags are
// skipped, and so is the separated VALUE of a value-taking flag, so a count
// is never mistaken for a file. A lone `-` is stdin. valued enables
// head/tail's flag grammar; for cat every dash word is simply dropped.
func readOperands(args []*syntax.Word, valued bool) []*syntax.Word {
	ops := []*syntax.Word{}
	skip, done := false, false
	for _, w := range args {
		if skip {
			skip = false
			continue
		}
		if done {
			ops = append(ops, w)
			continue
		}
		s, ok := literal(w)
		switch {
		case !ok:
			ops = append(ops, w)
		case s == "--":
			done = true
		case s == "-":
		case strings.HasPrefix(s, "--"):
			if valued && longValuedRead.MatchString(s) {
				skip = true
			}
		case strings.HasPrefix(s, "-") && len(s) > 1:
			if valued && shortValued.MatchString(s) {
				skip = true
			}
		case valued && oldStyleLimit.MatchString(s):
		default:
			ops = append(ops, w)
		}
	}
	return ops
}

func namesRealFile(ops []*syntax.Word) bool {
	for _, w := range ops {
		if s, ok := literal(w); ok && !magicPath.MatchString(s) {
			return true
		}
	}
	return false
}

// sedSuppressesOutput: a sed that suppresses automatic printing is SELECTING
// lines to print, which is a file read when it names a file. A sed without it
// transforms its input and is left alone.
func sedSuppressesOutput(args []*syntax.Word) bool {
	for _, w := range args {
		s, ok := literal(w)
		if ok && (sedQuietFlag.MatchString(s) || s == "--quiet" || s == "--silent") {
			return true
		}
	}
	return false
}

// sedFileOperands drops the flags and the leading script word, unless -e/-f
// already supplied the script, leaving every non-flag word a file.
func sedFileOperands(args []*syntax.Word) []*syntax.Word {
	scripted := false
	for _, w := range args {
		s, ok := literal(w)
		if ok && (sedScriptFlag.MatchString(s) ||
			strings.HasPrefix(s, "--expression") || strings.HasPrefix(s, "--file")) {
			scripted = true
		}
	}
	ops := []*syntax.Word{}
	for _, w := range args {
		if s, ok := literal(w); ok && strings.HasPrefix(s, "-") {
			continue
		}
		ops = append(ops, w)
	}
	if !scripted && len(ops) > 0 {
		ops = ops[1:]
	}
	return ops
}

func callIsSedSuppressing(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	return ok && e.name == "sed" && sedSuppressesOutput(c.Args[e.index+1:])
}

func callIsSedLineRead(c *syntax.CallExpr) bool {
	if !callIsSedSuppressing(c) {
		return false
	}
	e, _ := effectiveCommand(c)
	return namesRealFile(sedFileOperands(c.Args[e.index+1:]))
}

func callReadsBannedFile(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	if !ok || (e.name != "cat" && e.name != "head" && e.name != "tail") {
		return false
	}
	return namesRealFile(readOperands(c.Args[e.index+1:], e.name != "cat"))
}

func hasFileRead(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		return callReadsBannedFile(c) || callIsSedLineRead(c)
	})
}

// hasGitRM: the SUBCOMMAND is the leading non-flag word after `git`, so
// `git -C dir rm f` counts while `git commit -m rm` does not. `--cached`
// anywhere only unstages, and passes through.
func hasGitRM(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok || e.name != "git" {
			return false
		}
		sub, skip := "", false
		for _, w := range c.Args[e.index+1:] {
			s, st := literal(w)
			if st && s == "--cached" {
				return false
			}
			if sub != "" || skip {
				skip = false
				continue
			}
			switch {
			case !st:
				sub = "?"
			case s == "-C" || s == "-c":
				skip = true
			case strings.HasPrefix(s, "-"):
			default:
				sub = s
			}
		}
		return sub == "rm"
	})
}

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
