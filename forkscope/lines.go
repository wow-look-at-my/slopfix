package forkscope

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// Lines is the part of a fork's work tree that the fork wrote.
type Lines struct {
	// top is the work tree root, with every symlink resolved.
	top string
	// commit is the base the lines are measured from.
	commit string
	// whole holds each file new since the base, or untracked.
	whole set.Set[string]
	// lines holds, for each file the diff touched, the line numbers it added or changed.
	lines map[string]set.Set[int]
}

// Lines answers the lines of the work tree that differ from the base.
func (b *Base) Lines() (*Lines, error) {
	resolved, err := filepath.EvalSymlinks(b.top)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	own := &Lines{top: resolved, commit: b.commit, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	diff, err := gitIn(b.top, "-c", "core.quotePath=false", "diff", "-U0", "--no-renames", "--no-color", "--no-ext-diff",
		"--src-prefix=a/", "--dst-prefix=b/", b.commit, "--")
	if err != nil {
		return nil, fmt.Errorf("fork scope: diff against the base %s: %w", b.commit, err)
	}
	if err := own.readDiff(diff); err != nil {
		return nil, err
	}
	untracked, err := gitIn(b.top, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	for name := range strings.SplitSeq(untracked, "\x00") {
		if name != "" {
			own.whole.Add(filepath.ToSlash(name))
		}
	}
	if b.upstream != "" {
		if err := own.dropUpstreamVersions(b.top, b.upstream); err != nil {
			return nil, err
		}
	}
	return own, nil
}

// dropUpstreamVersions removes each clean file whose content a commit of
// upstream gave that path. An upstream sync brings such a file in whole, so
// the fork wrote none of it.
func (o *Lines) dropUpstreamVersions(top, upstream string) error {
	raw, err := gitIn(top, "log", upstream, "--format=", "--raw", "--no-abbrev", "--no-renames", "-z")
	if err != nil {
		return fmt.Errorf("fork scope: list the file versions of the upstream %s: %w", upstream, err)
	}
	known := set.New[string]()
	fields := strings.Split(raw, "\x00")
	for i := 0; i+1 < len(fields); i++ {
		head := strings.TrimSpace(fields[i])
		if !strings.HasPrefix(head, ":") {
			continue
		}
		if meta := strings.Fields(head[1:]); len(meta) >= 4 {
			known.Add(fields[i+1] + "\x00" + meta[3])
		}
		i++
	}
	dirty, err := gitIn(top, "diff", "--name-only", "-z", "HEAD", "--")
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	edited := set.New[string]()
	for name := range strings.SplitSeq(dirty, "\x00") {
		edited.Add(name)
	}
	staged, err := gitIn(top, "ls-files", "-s", "-z")
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	for entry := range strings.SplitSeq(staged, "\x00") {
		meta, name, ok := strings.Cut(entry, "\t")
		parts := strings.Fields(meta)
		if !ok || len(parts) < 2 || edited.Contains(name) || !known.Contains(name+"\x00"+parts[1]) {
			continue
		}
		delete(o.lines, name)
		o.whole.Remove(name)
	}
	return nil
}

// File answers the lines of text that the fork wrote, for text headed for path
// in the work tree. A path the base does not hold is the fork's whole.
func (b *Base) File(path, text string) (*Scope, error) {
	rel, err := relTo(b.top, path)
	if err != nil {
		return nil, err
	}
	listed, err := gitIn(b.top, "ls-tree", "-z", "--full-name", b.commit, "--", rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	if listed == "" {
		return Whole(), nil
	}
	upstream, err := gitIn(b.top, "cat-file", "blob", b.commit+":"+rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: read %s at the base: %w", rel, err)
	}
	return Changed(upstream, text), nil
}

// relTo answers path relative to the work tree at top, in slash form. A path
// outside it is an error. The links of the part of path that exists resolve,
// so a path that does not exist yet is placed by its nearest directory.
func relTo(top, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	head, tail := abs, ""
	for {
		if resolved, err := filepath.EvalSymlinks(head); err == nil {
			abs = filepath.Join(resolved, tail)
			break
		}
		parent := filepath.Dir(head)
		if parent == head {
			break
		}
		tail = filepath.Join(filepath.Base(head), tail)
		head = parent
	}
	rel, err := filepath.Rel(realPath(top), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fork scope: %s is outside the work tree %s", path, top)
	}
	return filepath.ToSlash(rel), nil
}

// Changed answers the lines of after that differ from before.
func Changed(before, after string) *Scope {
	s := &Scope{base: func() (string, error) { return before, nil }}
	for _, op := range opcodes(before, after) {
		if op.Tag != 'r' && op.Tag != 'i' {
			continue
		}
		for j := op.J1; j < op.J2; j++ {
			s.lines.Add(j + 1)
		}
	}
	return s
}

// readDiff records what a zero-context diff added. A hunk header gives the
// count of lines on each side, so a content line is never read as a header.
func (o *Lines) readDiff(diff string) error {
	scanner := bufio.NewScanner(strings.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var file string
	created := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file, created = "", false
		case line == "--- /dev/null":
			created = true
		case strings.HasPrefix(line, "+++ "):
			name, err := diffPath(strings.TrimPrefix(line, "+++ "))
			if err != nil {
				return err
			}
			file = name
			if file != "" && created {
				o.whole.Add(file)
			}
		case strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ"):
			name, err := binaryDiffPath(strings.TrimSuffix(strings.TrimPrefix(line, "Binary files "), " differ"))
			if err != nil {
				return err
			}
			if name != "" {
				o.whole.Add(name)
			}
		case strings.HasPrefix(line, "@@ "):
			start, count, err := hunkNew(line)
			if err != nil {
				return err
			}
			if err := o.addHunk(scanner, file, start, count, hunkOldCount(line)); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// addHunk records a hunk's new lines and reads past its body.
func (o *Lines) addHunk(scanner *bufio.Scanner, file string, start, added, removed int) error {
	if file != "" && added > 0 {
		lines, ok := o.lines[file]
		if !ok {
			lines = set.New[int]()
			o.lines[file] = lines
		}
		for n := start; n < start+added; n++ {
			lines.Add(n)
		}
	}
	for added > 0 || removed > 0 {
		if !scanner.Scan() {
			return fmt.Errorf("fork scope: the diff of %s ends inside a hunk", file)
		}
		switch body := scanner.Text(); {
		case strings.HasPrefix(body, "+"):
			added--
		case strings.HasPrefix(body, "-"):
			removed--
		}
	}
	return nil
}

// diffPath answers the path a "+++" header names, or "" for /dev/null.
func diffPath(field string) (string, error) {
	if field == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(field, `"`) {
		unquoted, err := strconv.Unquote(field)
		if err != nil {
			return "", fmt.Errorf("fork scope: the diff names %s, which does not unquote: %w", field, err)
		}
		field = unquoted
	}
	return strings.TrimPrefix(field, "b/"), nil
}

// binaryDiffPath answers the new path of a binary diff's "A and B" pair, or ""
// for a deleted file. The diff runs with --no-renames, so A and B name one path
// unless a side is /dev/null. The halves therefore split at the middle " and ".
func binaryDiffPath(pair string) (string, error) {
	if newSide, ok := strings.CutPrefix(pair, "/dev/null and "); ok {
		return diffPath(newSide)
	}
	if strings.HasSuffix(pair, " and /dev/null") {
		return "", nil
	}
	refused := fmt.Errorf("fork scope: the binary diff %q does not name one path on each side", pair)
	half := (len(pair) - len(" and ")) / 2
	if half <= 0 || pair[half:half+len(" and ")] != " and " {
		return "", refused
	}
	oldSide, err := diffPath(strings.Replace(pair[:half], "a/", "b/", 1))
	if err != nil {
		return "", err
	}
	newSide, err := diffPath(pair[half+len(" and "):])
	if err != nil {
		return "", err
	}
	if oldSide != newSide {
		return "", refused
	}
	return newSide, nil
}

// hunkRange reads "start[,count]" after the sign of a hunk header side.
func hunkRange(side string) (start, count int, err error) {
	first, rest, ranged := strings.Cut(side, ",")
	if start, err = strconv.Atoi(first); err != nil {
		return 0, 0, err
	}
	if !ranged {
		return start, 1, nil
	}
	count, err = strconv.Atoi(rest)
	return start, count, err
}

// hunkNew answers the first new line and the new line count of "@@ -a,b +c,d @@".
func hunkNew(header string) (start, count int, err error) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, fmt.Errorf("fork scope: %q is not a hunk header", header)
	}
	start, count, err = hunkRange(strings.TrimPrefix(fields[2], "+"))
	if err != nil {
		return 0, 0, fmt.Errorf("fork scope: %q is not a hunk header: %w", header, err)
	}
	return start, count, nil
}

// hunkOldCount answers the line count of a header hunkNew already read.
func hunkOldCount(header string) int {
	_, count, _ := hunkRange(strings.TrimPrefix(strings.Fields(header)[1], "-"))
	return count
}

// rel answers path relative to the work tree, in slash form.
func (o *Lines) rel(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(o.top, abs)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// Scope answers the lines of path that the fork wrote. A file the fork never
// touched answers an empty Scope.
func (o *Lines) Scope(path string) *Scope {
	name := o.rel(path)
	if o.whole.Contains(name) {
		return Whole()
	}
	out := &Scope{lines: o.lines[name]}
	// A selector measures from no base commit, so it reads no base text.
	if o.commit != "" {
		out.base = func() (string, error) {
			text, err := gitIn(o.top, "cat-file", "blob", o.commit+":"+name)
			if err != nil {
				return "", fmt.Errorf("fork scope: read %s at the base: %w", name, err)
			}
			return text, nil
		}
	}
	return out
}

// DiffLines answers the lines a selector changed in the work tree that holds
// dir. With staged it is the index against HEAD, and otherwise the work tree
// against rev. A path the diff created counts whole.
func DiffLines(dir, rev string, staged bool) (*Lines, error) {
	top, err := gitIn(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	top = strings.TrimSpace(top)
	resolved, err := filepath.EvalSymlinks(top)
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	args := []string{"-c", "core.quotePath=false", "diff", "-U0", "--no-renames", "--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/"}
	if staged {
		args = append(args, "--cached")
	} else {
		args = append(args, rev)
	}
	args = append(args, "--")
	diff, err := gitIn(top, args...)
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	own := &Lines{top: resolved, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	if err := own.readDiff(diff); err != nil {
		return nil, err
	}
	return own, nil
}

// IntersectLines answers the lines both a and b hold. A file both hold whole
// stays whole, and a file one holds whole takes the other's lines.
func IntersectLines(a, b *Lines) *Lines {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	out := &Lines{top: a.top, commit: a.commit, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	names := set.New[string]()
	for name := range a.whole.All() {
		names.Add(name)
	}
	for name := range b.whole.All() {
		names.Add(name)
	}
	for name := range a.lines {
		names.Add(name)
	}
	for name := range b.lines {
		names.Add(name)
	}
	for name := range names.All() {
		aWhole, bWhole := a.whole.Contains(name), b.whole.Contains(name)
		switch {
		case aWhole && bWhole:
			out.whole.Add(name)
		case aWhole:
			copyLines(out, name, b.lines[name])
		case bWhole:
			copyLines(out, name, a.lines[name])
		default:
			copyLines(out, name, a.lines[name].Intersection(b.lines[name]))
		}
	}
	return out
}

// copyLines records the lines of name when it holds any.
func copyLines(o *Lines, name string, lines set.Set[int]) {
	if lines.Len() > 0 {
		o.lines[name] = lines
	}
}

// Whole reports whether the fork wrote all of path: it is new since the base, or untracked.
func (o *Lines) Whole(path string) bool {
	return o.whole.Contains(o.rel(path))
}

// Holds reports whether the fork wrote any line from first to last of path.
func (o *Lines) Holds(path string, first, last int) bool {
	return o.Scope(path).Holds(first, last)
}
