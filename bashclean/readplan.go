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

// ReadArgs is the input of one Read tool call.
type ReadArgs struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ReadPlan is the Read calls that print what a Bash file read prints, and the
// note the model reads after their result.
type ReadPlan struct {
	Reads []ReadArgs `json:"reads"`
	Note  string     `json:"note"`
}

var (
	sedRange     = regexp.MustCompile(`^([0-9]+)(?:,([0-9]+))?p$`)
	lineCount    = regexp.MustCompile(`^\+?[0-9]+$`)
	attachedN    = regexp.MustCompile(`^-n(\+?[0-9]+)$`)
	oldStyleN    = regexp.MustCompile(`^-([0-9]+)$`)
	longLinesArg = regexp.MustCompile(`^--lines=(\+?[0-9]+)$`)
)

// defaultLines is the count head and tail print with no count flag.
const defaultLines = 10

// maxCountBytes bounds the file a tail count reads for its line total.
const maxCountBytes = 10 << 20

// readSpec is one planned Read. fromEnd is a tail count, which needs the
// line total of the file before it is an offset.
type readSpec struct {
	path          string
	offset, limit int
	fromEnd       int
}

// PlanRead maps one plain `cat`, `head`, `tail` or `sed -n` call onto one Read
// for each file. dir is the directory the command runs in, because Read takes
// an absolute path. A command with no exact Read equivalent returns nil, and
// the caller runs it as written.
func PlanRead(command, dir string) *ReadPlan {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil || len(f.Stmts) != 1 {
		return nil
	}
	s := f.Stmts[0]
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(s.Redirs) > 0 || s.Background || s.Negated || s.Coprocess || len(c.Assigns) > 0 {
		return nil
	}
	e, ok := effectiveCommand(c)
	if !ok {
		return nil
	}
	args := []string{}
	for _, w := range c.Args[e.index+1:] {
		v, ok := literal(w)
		if !ok || strings.ContainsAny(v, "*?[") {
			return nil
		}
		args = append(args, v)
	}
	var specs []readSpec
	switch e.name {
	case "cat":
		specs = catSpecs(args)
	case "head", "tail":
		specs = headTailSpecs(e.name == "tail", args)
	case "sed":
		specs = sedSpecs(args)
	}
	if specs == nil {
		return nil
	}
	plan := &ReadPlan{}
	for _, sp := range specs {
		r, ok := resolve(sp, dir)
		if !ok {
			return nil
		}
		plan.Reads = append(plan.Reads, r)
	}
	plan.Note = readNote(command, plan.Reads)
	return plan
}

func catSpecs(args []string) []readSpec {
	out := []readSpec{}
	for _, a := range args {
		switch {
		case a == "-n" || a == "--number":
		case strings.HasPrefix(a, "-"):
			return nil
		default:
			out = append(out, readSpec{path: a})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// headTailSpecs reads the count in each spelling head and tail accept. Every
// file gets the same count, as head and tail give it.
func headTailSpecs(tail bool, args []string) []readSpec {
	count := strconv.Itoa(defaultLines)
	files := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		m := attachedN.FindStringSubmatch(a)
		if m == nil {
			m = longLinesArg.FindStringSubmatch(a)
		}
		if m == nil {
			m = oldStyleN.FindStringSubmatch(a)
		}
		switch {
		case (a == "-n" || a == "--lines") && i+1 < len(args) && len(files) == 0:
			i++
			count = args[i]
		case m != nil && len(files) == 0:
			count = m[1]
		case strings.HasPrefix(a, "-"):
			return nil
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 || !lineCount.MatchString(count) {
		return nil
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(count, "+"))
	fromStart := strings.HasPrefix(count, "+")
	if n == 0 || (!tail && fromStart) {
		return nil
	}
	out := []readSpec{}
	for _, f := range files {
		switch {
		case !tail:
			out = append(out, readSpec{path: f, limit: n})
		case fromStart:
			out = append(out, readSpec{path: f, offset: n})
		default:
			out = append(out, readSpec{path: f, fromEnd: n})
		}
	}
	return out
}

// sedSpecs maps `sed -n 'Np'` and `sed -n 'A,Bp'` on one file. sed numbers
// lines across all its files as one stream, so a second file does not map.
func sedSpecs(args []string) []readSpec {
	quiet := false
	rest := []string{}
	for _, a := range args {
		switch {
		case a == "-n" || a == "--quiet" || a == "--silent":
			quiet = true
		case strings.HasPrefix(a, "-"):
			return nil
		default:
			rest = append(rest, a)
		}
	}
	if !quiet || len(rest) != 2 {
		return nil
	}
	m := sedRange.FindStringSubmatch(rest[0])
	if m == nil {
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
	return []readSpec{{path: rest[1], offset: a, limit: b - a + 1}}
}

// resolve makes the path absolute and turns a tail count into an offset.
func resolve(sp readSpec, dir string) (ReadArgs, bool) {
	p, ok := readPath(sp.path, dir)
	if !ok {
		return ReadArgs{}, false
	}
	r := ReadArgs{FilePath: p, Offset: sp.offset, Limit: sp.limit}
	if sp.fromEnd == 0 {
		return r, true
	}
	total, ok := countLines(p)
	if !ok {
		return ReadArgs{}, false
	}
	r.Offset = max(1, total-sp.fromEnd+1)
	r.Limit = min(sp.fromEnd, total)
	return r, true
}

// readPath makes a file operand absolute. A pseudo-file under /proc, /sys or
// /dev is left to Bash, and a relative path with no directory has no
// absolute form.
func readPath(p, dir string) (string, bool) {
	if p == "" || p == "-" {
		return "", false
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if !filepath.IsAbs(p) {
		if dir == "" || !filepath.IsAbs(dir) {
			return "", false
		}
		p = filepath.Join(dir, p)
	}
	p = filepath.Clean(p)
	if magicPath.MatchString(p) {
		return "", false
	}
	return p, true
}

// countLines counts lines the way Read numbers them: a last line with no
// newline still counts.
func countLines(p string) (int, bool) {
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxCountBytes {
		return 0, false
	}
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

// readNote tells the model what ran in place of its command, and which tool
// to reach for next time.
func readNote(command string, reads []ReadArgs) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Your Bash command `%s` read a file, so it did not run. The Read tool answered it with:\n", command)
	for _, r := range reads {
		fmt.Fprintf(&b, "- Read(file_path: %s", strconv.Quote(r.FilePath))
		if r.Offset > 0 {
			fmt.Fprintf(&b, ", offset: %d", r.Offset)
		}
		if r.Limit > 0 {
			fmt.Fprintf(&b, ", limit: %d", r.Limit)
		}
		b.WriteString(")\n")
	}
	b.WriteString("Use the Read tool to read files. Its offset and limit parameters select lines, which is what head, tail and sed -n were for.")
	return b.String()
}
