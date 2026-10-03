package bashclean

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var (
	sedRange     = regexp.MustCompile(`^([0-9]+)(?:,([0-9]+))?p$`)
	lineCount    = regexp.MustCompile(`^\+?[0-9]+$`)
	attachedN    = regexp.MustCompile(`^-n(\+?[0-9]+)$`)
	oldStyleN    = regexp.MustCompile(`^-([0-9]+)$`)
	longLinesArg = regexp.MustCompile(`^--lines=(\+?[0-9]+)$`)
)

// defaultLines is the count head and tail print with no count flag.
const defaultLines = 10

// readCalls gives the Read calls that print what a file-read command prints.
// It answers only for one plain call with no pipe and no redirect, because
// the deny must not name a call that reads something else. Read takes an
// absolute path, so a relative path needs dir. Any doubt returns nil, and
// the deny then gives the general text alone.
func readCalls(f *syntax.File, dir string) []string {
	if len(f.Stmts) != 1 {
		return nil
	}
	s := f.Stmts[0]
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(s.Redirs) > 0 || s.Background || s.Negated || len(c.Assigns) > 0 {
		return nil
	}
	e, ok := effectiveCommand(c)
	if !ok {
		return nil
	}
	args := []string{}
	for _, w := range c.Args[e.index+1:] {
		v, ok := literal(w)
		if !ok {
			return nil
		}
		args = append(args, v)
	}
	switch e.name {
	case "cat":
		return catCalls(args, dir)
	case "head", "tail":
		return headTailCall(e.name == "tail", args, dir)
	case "sed":
		return sedCall(args, dir)
	}
	return nil
}

func catCalls(args []string, dir string) []string {
	out := []string{}
	for _, a := range args {
		if a == "-n" || a == "--number" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			return nil
		}
		p, ok := readPath(a, dir)
		if !ok {
			return nil
		}
		out = append(out, formatRead(p, 0, 0))
	}
	return out
}

// headTailCall reads the count in each spelling head and tail accept. A
// tail count without a plus sign counts from the end, so the call needs the
// line total of the file.
func headTailCall(tail bool, args []string, dir string) []string {
	count, file := strconv.Itoa(defaultLines), ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch m := []string{}; {
		case (a == "-n" || a == "--lines") && i+1 < len(args):
			i++
			count = args[i]
		case matchInto(attachedN, a, &m), matchInto(longLinesArg, a, &m), matchInto(oldStyleN, a, &m):
			count = m[1]
		case strings.HasPrefix(a, "-") || file != "":
			return nil
		default:
			file = a
		}
	}
	if file == "" || !lineCount.MatchString(count) {
		return nil
	}
	p, ok := readPath(file, dir)
	if !ok {
		return nil
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(count, "+"))
	switch {
	case !tail && strings.HasPrefix(count, "+"), n == 0:
		return nil
	case !tail:
		return []string{formatRead(p, 0, n)}
	case strings.HasPrefix(count, "+"):
		return []string{formatRead(p, n, 0)}
	}
	total, ok := countLines(p)
	if !ok {
		return nil
	}
	return []string{formatRead(p, max(1, total-n+1), min(n, total))}
}

// sedCall maps `sed -n 'Np' file` and `sed -n 'A,Bp' file`. A script that
// does anything but print one range is not a read of lines.
func sedCall(args []string, dir string) []string {
	quiet, script, file := false, "", ""
	for _, a := range args {
		switch {
		case a == "-n" || a == "--quiet" || a == "--silent":
			quiet = true
		case strings.HasPrefix(a, "-"):
			return nil
		case script == "":
			script = a
		case file == "":
			file = a
		default:
			return nil
		}
	}
	m := sedRange.FindStringSubmatch(script)
	if !quiet || m == nil || file == "" {
		return nil
	}
	a, _ := strconv.Atoi(m[1])
	b := a
	if m[2] != "" {
		b, _ = strconv.Atoi(m[2])
	}
	if a == 0 || b < a {
		return nil
	}
	p, ok := readPath(file, dir)
	if !ok {
		return nil
	}
	return []string{formatRead(p, a, b-a+1)}
}

func matchInto(re *regexp.Regexp, s string, m *[]string) bool {
	*m = re.FindStringSubmatch(s)
	return *m != nil
}

// readPath makes a file operand absolute. A pseudo-file is not denied at
// all, and a relative path with no directory has no absolute form.
func readPath(p, dir string) (string, bool) {
	if p == "" || p == "-" || magicPath.MatchString(p) {
		return "", false
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		p = filepath.Join(home, p[2:])
	}
	if !filepath.IsAbs(p) {
		if dir == "" {
			return "", false
		}
		p = filepath.Join(dir, p)
	}
	return filepath.Clean(p), true
}

// countLines counts lines the way Read numbers them: a last line with no
// newline still counts.
func countLines(p string) (int, bool) {
	data, err := os.ReadFile(p)
	if err != nil {
		return 0, false
	}
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n, true
}

// formatRead writes a call the way the model writes one.
func formatRead(p string, offset, limit int) string {
	s := "Read(file_path=" + strconv.Quote(p)
	if offset > 0 {
		s += fmt.Sprintf(", offset=%d", offset)
	}
	if limit > 0 {
		s += fmt.Sprintf(", limit=%d", limit)
	}
	return s + ")"
}
