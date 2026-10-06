package forkscope

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// Lines is the part of a fork's work tree that the fork wrote.
type Lines struct {
	// top is the work tree root, with every symlink resolved.
	top string
	// repo reads the base commit's objects in-process.
	repo *gitread.Repo
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
	own := &Lines{top: resolved, repo: b.repo, commit: b.commit, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	if err := own.readWorkTree(); err != nil {
		return nil, err
	}
	if b.upstream != "" {
		if err := own.dropUpstreamVersions(b.upstream); err != nil {
			return nil, err
		}
	}
	return own, nil
}

// readWorkTree records every tracked file that differs from the base and every
// untracked file, which is what a zero-context diff against the base names.
func (o *Lines) readWorkTree() error {
	base, err := gitread.ParseOID(o.commit)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	base, err = o.repo.PeelCommit(base)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	baseNames, err := o.repo.TreeNames(base)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	idx, err := o.repo.Index()
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	tracked := idx.Tracked()
	for path := range tracked {
		file := filepath.Join(o.top, filepath.FromSlash(path))
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		baseEntry, ok := baseNames[path]
		if !ok {
			o.whole.Add(path)
			continue
		}
		if baseEntry.Mode == "160000" {
			continue
		}
		before, ok := o.repo.Blob(baseEntry.OID)
		if !ok {
			continue
		}
		if bytes.Equal(before, data) {
			continue
		}
		if isBinary(before) || isBinary(data) {
			o.whole.Add(path)
			continue
		}
		o.recordLines(path, string(before), string(data))
	}
	return o.addUntracked(tracked)
}

// recordLines adds the new line numbers a change to a file holds.
func (o *Lines) recordLines(path, before, after string) {
	scope := Changed(before, after)
	if scope.Empty() {
		return
	}
	o.lines[path] = scope.lines
}

// addUntracked records every file the work tree holds that the index does not,
// and that git does not ignore.
func (o *Lines) addUntracked(tracked map[string]gitread.IndexEntry) error {
	ignore := o.repo.Ignore()
	return filepath.WalkDir(o.top, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(o.top, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path == o.top || rel == ".git" || strings.HasPrefix(rel, ".git/") {
				return nil
			}
			if entry, ok := tracked[rel]; ok && entry.Mode == "160000" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
				return filepath.SkipDir
			}
			if ignore.Ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := tracked[rel]; ok {
			return nil
		}
		if ignore.Ignored(rel, false) {
			return nil
		}
		o.whole.Add(rel)
		return nil
	})
}

// isBinary reports whether content holds a NUL byte, which git reads as binary.
func isBinary(data []byte) bool { return bytes.IndexByte(data, 0) >= 0 }

// dropUpstreamVersions removes each clean file whose content a commit of
// upstream gave that path. An upstream sync brings such a file in whole, so
// the fork wrote none of it.
func (o *Lines) dropUpstreamVersions(upstream string) error {
	tip, err := gitread.ParseOID(upstream)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	commits, err := o.repo.RevList(tip)
	if err != nil {
		return fmt.Errorf("fork scope: list the file versions of the upstream %s: %w", upstream, err)
	}
	known := set.New[string]()
	for _, commit := range commits {
		names, err := o.repo.TreeNames(commit)
		if err != nil {
			return fmt.Errorf("fork scope: list the file versions of the upstream %s: %w", upstream, err)
		}
		for path, entry := range names {
			known.Add(path + "\x00" + entry.OID.String())
		}
	}
	dirty, err := o.dirtyPaths()
	if err != nil {
		return err
	}
	idx, err := o.repo.Index()
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	for _, entry := range idx.Entries() {
		if dirty.Contains(entry.Path) || !known.Contains(entry.Path+"\x00"+entry.OID.String()) {
			continue
		}
		delete(o.lines, entry.Path)
		o.whole.Remove(entry.Path)
	}
	return nil
}

// dirtyPaths answers every tracked path whose work-tree content differs from HEAD.
func (o *Lines) dirtyPaths() (set.Set[string], error) {
	out := set.New[string]()
	head, err := o.repo.Head()
	if err != nil {
		return out, fmt.Errorf("fork scope: %w", err)
	}
	headNames, err := o.repo.TreeNames(head)
	if err != nil {
		return out, fmt.Errorf("fork scope: %w", err)
	}
	idx, err := o.repo.Index()
	if err != nil {
		return out, fmt.Errorf("fork scope: %w", err)
	}
	for path := range idx.Tracked() {
		entry, ok := headNames[path]
		if !ok {
			out.Add(path)
			continue
		}
		data, err := os.ReadFile(filepath.Join(o.top, filepath.FromSlash(path)))
		if err != nil {
			out.Add(path)
			continue
		}
		before, ok := o.repo.Blob(entry.OID)
		if !ok || !bytes.Equal(before, data) {
			out.Add(path)
		}
	}
	return out, nil
}

// File answers the lines of text that the fork wrote, for text headed for path
// in the work tree. A path the base does not hold is the fork's whole.
func (b *Base) File(path, text string) (*Scope, error) {
	rel, err := relTo(b.top, path)
	if err != nil {
		return nil, err
	}
	commit, err := gitread.ParseOID(b.commit)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	entry, ok, err := b.repo.TreeEntryAt(commit, rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	if !ok || entry.Mode == "160000" {
		return Whole(), nil
	}
	upstream, ok := b.repo.Blob(entry.OID)
	if !ok {
		return nil, fmt.Errorf("fork scope: read %s at the base: the blob is missing", rel)
	}
	return Changed(string(upstream), text), nil
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
	if o.commit != "" && o.repo != nil {
		commit, err := gitread.ParseOID(o.commit)
		repo := o.repo
		if err == nil {
			out.base = func() (string, error) {
				text, err := repo.BlobAt(commit, name)
				if err != nil {
					return "", fmt.Errorf("fork scope: read %s at the base: %w", name, err)
				}
				return string(text), nil
			}
		}
	}
	return out
}

// DiffLines answers the lines a selector changed in the work tree that holds
// dir. With staged it is the index against HEAD, and otherwise the work tree
// against rev. A path the diff created counts whole.
func DiffLines(dir, rev string, staged bool) (*Lines, error) {
	repo, err := gitread.OpenWorkTree(dir)
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("diff scope: git rev-parse --show-toplevel: %s is not inside a work tree", dir)
	}
	top, err := filepath.EvalSymlinks(repo.WorkTree())
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	own := &Lines{top: top, repo: repo, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	idx, err := repo.Index()
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	if staged {
		if err := own.readStaged(repo, idx); err != nil {
			return nil, err
		}
		return own, nil
	}
	if err := own.readAgainst(repo, idx, rev); err != nil {
		return nil, err
	}
	return own, nil
}

// readStaged records the lines the index changed against HEAD.
func (o *Lines) readStaged(repo *gitread.Repo, idx *gitread.Index) error {
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("diff scope: %w", err)
	}
	headNames, err := repo.TreeNames(head)
	if err != nil {
		return fmt.Errorf("diff scope: %w", err)
	}
	for path, entry := range idx.Tracked() {
		before, ok := headNames[path]
		if !ok {
			o.whole.Add(path)
			continue
		}
		if before.OID == entry.OID {
			continue
		}
		old, okOld := repo.Blob(before.OID)
		next, okNext := repo.Blob(entry.OID)
		if !okOld || !okNext {
			continue
		}
		if isBinary(old) || isBinary(next) {
			o.whole.Add(path)
			continue
		}
		o.recordLines(path, string(old), string(next))
	}
	return nil
}

// readAgainst records the lines the work tree changed against a revision.
func (o *Lines) readAgainst(repo *gitread.Repo, idx *gitread.Index, rev string) error {
	base, err := resolveRev(repo, rev)
	if err != nil {
		return fmt.Errorf("diff scope: %w", err)
	}
	baseNames, err := repo.TreeNames(base)
	if err != nil {
		return fmt.Errorf("diff scope: %w", err)
	}
	for path := range idx.Tracked() {
		data, err := os.ReadFile(filepath.Join(o.top, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		entry, ok := baseNames[path]
		if !ok {
			o.whole.Add(path)
			continue
		}
		if entry.Mode == "160000" {
			continue
		}
		before, ok := repo.Blob(entry.OID)
		if !ok || bytes.Equal(before, data) {
			continue
		}
		if isBinary(before) || isBinary(data) {
			o.whole.Add(path)
			continue
		}
		o.recordLines(path, string(before), string(data))
	}
	return nil
}

// resolveRev answers the commit a revision string names.
func resolveRev(repo *gitread.Repo, rev string) (gitread.OID, error) {
	if rev == "" || rev == "HEAD" {
		return repo.Head()
	}
	oid, err := repo.Resolve(rev)
	if err != nil {
		return gitread.OID{}, err
	}
	return repo.PeelCommit(oid)
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
	out := &Lines{top: a.top, repo: a.repo, commit: a.commit, whole: set.New[string](), lines: map[string]set.Set[int]{}}
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

// Claim records path as a file the fork wrote all of.
func (o *Lines) Claim(path string) {
	o.whole.Add(o.rel(path))
}

// Holds reports whether the fork wrote any line from first to last of path.
func (o *Lines) Holds(path string, first, last int) bool {
	return o.Scope(path).Holds(first, last)
}
