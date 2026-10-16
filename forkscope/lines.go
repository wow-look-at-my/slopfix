package forkscope

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
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
	base, repo, err := resolve(b.top, b.commit)
	if err != nil {
		return nil, err
	}
	if err := own.readWorkTree(repo, base, Changed, true); err != nil {
		return nil, fmt.Errorf("fork scope: diff against the base %s: %w", b.commit, err)
	}
	if b.upstream != "" {
		if err := own.dropUpstreamVersions(repo, b.upstream); err != nil {
			return nil, err
		}
	}
	return own, nil
}

// pair is a tracked file that differs from the base: its base blob and its new text.
type pair struct {
	name   string
	before gitread.OID
	after  []byte
}

// readWorkTree records each tracked file whose work tree copy differs from
// base, as a diff of the work tree against base names it. With untracked, it
// records each untracked file git does not ignore as whole too.
func (o *Lines) readWorkTree(repo *gitread.Repo, base gitread.OID, lines func(before, after string) *Scope, untracked bool) error {
	idx, err := repo.Index()
	if err != nil {
		return err
	}
	changed, others, err := repo.Changes(base)
	if err != nil {
		return err
	}
	if untracked {
		for _, name := range others {
			o.whole.Add(name)
		}
	}
	unmerged := set.New[string]()
	for _, e := range idx.Entries() {
		if e.Stage != 0 {
			unmerged.Add(e.Path)
		}
	}
	var pairs []pair
	for _, name := range changed {
		// A path the index dropped is a deletion, which adds no line.
		entry, ok := idx.Tracked()[name]
		if ok && entry.IsGitlink() || !ok && !unmerged.Contains(name) {
			continue
		}
		after, err := readWorkFile(repo.WorkTree(), name)
		if err != nil {
			// The file is gone, so the diff deletes it and adds no line.
			continue
		}
		before, ok, err := repo.TreeEntryAt(base, name)
		if err != nil {
			return err
		}
		if !ok || before.IsDir() {
			o.whole.Add(name)
			continue
		}
		pairs = append(pairs, pair{name, before.OID, after})
	}
	return o.addPairs(repo, pairs, lines)
}

// addPairs records the lines each pair's new text adds. A binary file counts whole.
func (o *Lines) addPairs(repo *gitread.Repo, pairs []pair, lines func(before, after string) *Scope) error {
	oids := make([]gitread.OID, len(pairs))
	for i, p := range pairs {
		oids[i] = p.before
	}
	blobs, err := readBlobs(o.top, repo, oids)
	if err != nil {
		return err
	}
	attrs := repo.Attributes()
	for _, p := range pairs {
		before := blobs[p.before]
		if bytes.Equal(before, p.after) {
			continue
		}
		if isBinary(before) || isBinary(p.after) || attrs.Value(p.name, "diff") == gitread.AttrUnset {
			o.whole.Add(p.name)
			continue
		}
		if scope := lines(string(before), string(p.after)); scope.lines.Len() > 0 {
			o.lines[p.name] = scope.lines
		}
	}
	return nil
}

// binarySniff is how far into a file git looks for a NUL byte.
const binarySniff = 8000

// isBinary reports content git's diff treats as binary: a NUL byte near its start.
func isBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), binarySniff)], 0) >= 0
}

// readWorkFile reads a work tree file, or the target of a symlink, as git hashes it.
func readWorkFile(top, name string) ([]byte, error) {
	path := filepath.Join(top, filepath.FromSlash(name))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		return []byte(filepath.ToSlash(target)), err
	}
	return os.ReadFile(path)
}

// readBlobs answers the content of each blob. A partial clone leaves blobs on
// its promisor remote, so the missing ones are fetched in one git call, as git
// diff fetches them.
func readBlobs(top string, repo *gitread.Repo, oids []gitread.OID) (map[gitread.OID][]byte, error) {
	out := map[gitread.OID][]byte{}
	var missing []gitread.OID
	for _, oid := range oids {
		if data, ok := repo.Blob(oid); ok {
			out[oid] = data
		} else {
			missing = append(missing, oid)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	if err := fetchBlobs(top, repo, missing); err != nil {
		return nil, err
	}
	for _, oid := range missing {
		data, ok := repo.Blob(oid)
		if !ok {
			return nil, fmt.Errorf("fork scope: the blob %s is not in the repository, and its promisor remote did not supply it", oid)
		}
		out[oid] = data
	}
	return out, nil
}

// fetchBlobs asks the promisor remote for blobs by name, the call git makes
// for a lazy fetch.
func fetchBlobs(top string, repo *gitread.Repo, oids []gitread.OID) error {
	remote := repo.PromisorRemote()
	if remote == "" {
		return fmt.Errorf("fork scope: the blob %s is not in the repository, and no promisor remote holds it", oids[0])
	}
	names := make([]string, len(oids))
	for i, oid := range oids {
		names[i] = oid.String()
	}
	cmd := exec.Command("git", "-c", "fetch.negotiationAlgorithm=noop", "fetch", remote, "--quiet", "--no-tags",
		"--no-write-fetch-head", "--recurse-submodules=no", "--filter=blob:none", "--stdin")
	cmd.Dir = top
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fork scope: fetch the missing blobs from %s: %w: %s", remote, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// blobAt answers the text of rel at commit, or false when commit holds no such file.
func blobAt(top, commit, rel string) (string, bool, error) {
	oid, repo, err := resolve(top, commit)
	if err != nil {
		return "", false, err
	}
	entry, ok, err := repo.TreeEntryAt(oid, rel)
	if err != nil {
		return "", false, fmt.Errorf("fork scope: %w", err)
	}
	if !ok || entry.IsDir() {
		return "", false, nil
	}
	blobs, err := readBlobs(top, repo, []gitread.OID{entry.OID})
	if err != nil {
		return "", false, err
	}
	return string(blobs[entry.OID]), true, nil
}

// dropUpstreamVersions removes each clean file whose content a commit of
// upstream gave that path. An upstream sync brings such a file in whole, so
// the fork wrote none of it.
func (o *Lines) dropUpstreamVersions(repo *gitread.Repo, upstream string) error {
	tip, _, err := resolve(o.top, upstream)
	if err != nil {
		return err
	}
	head, _, err := resolve(o.top, "HEAD")
	if err != nil {
		return err
	}
	dirty, _, err := repo.Changes(head)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	edited := set.Of(dirty...)
	idx, err := repo.Index()
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	// Only a clean staged file can be dropped, so the walk reads only the directories that hold one.
	staged := set.New[string]()
	dirs := set.Of("")
	for _, e := range idx.Entries() {
		if edited.Contains(e.Path) {
			continue
		}
		staged.Add(e.Path + "\x00" + e.OID.String())
		for dir := path.Dir(e.Path); dir != "." && !dirs.Contains(dir); dir = path.Dir(dir) {
			dirs.Add(dir)
		}
	}
	known := set.New[string]()
	err = repo.HistoryVersions(tip, dirs.Contains, func(name string, blob gitread.OID) {
		if staged.Contains(name + "\x00" + blob.String()) {
			known.Add(name)
		}
	})
	if err != nil {
		return fmt.Errorf("fork scope: list the file versions of the upstream %s: %w", upstream, err)
	}
	for name := range known.All() {
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
	upstream, ok, err := blobAt(b.top, b.commit, rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: read %s at the base: %w", rel, err)
	}
	if !ok {
		return Whole(), nil
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
	return added(before, after).ClaimWordless(after)
}

// added answers the lines of after that a diff from before inserts or replaces.
func added(before, after string) *Scope {
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

// ClaimWordless answers s with each line of text that holds no word, where the
// nearest lines with words above and below are both the fork's. Such a line
// matches any blank line of the base, so the match says nothing about who wrote it.
func (s *Scope) ClaimWordless(text string) *Scope {
	if s == nil || s.whole {
		return s
	}
	out := &Scope{lines: set.New[int](), base: s.base}
	for n := range s.lines.All() {
		out.lines.Add(n)
	}
	claimWordless(out, splitLines(text))
	return out
}

func claimWordless(s *Scope, lines []string) {
	wordless := func(line string) bool {
		return strings.IndexFunc(line, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) < 0
	}
	for i, line := range lines {
		if s.lines.Contains(i+1) || !wordless(line) {
			continue
		}
		above, below := i-1, i+1
		for above >= 0 && wordless(lines[above]) {
			above--
		}
		for below < len(lines) && wordless(lines[below]) {
			below++
		}
		if above >= 0 && below < len(lines) && s.lines.Contains(above+1) && s.lines.Contains(below+1) {
			s.lines.Add(i + 1)
		}
	}
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
			text, ok, err := blobAt(o.top, o.commit, name)
			if err == nil && !ok {
				err = fmt.Errorf("the base holds no such file")
			}
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
	repo, err := gitread.OpenWorkTree(dir)
	if err == nil && repo == nil {
		err = fmt.Errorf("%s is not inside a work tree", dir)
	}
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	own := &Lines{top: repo.WorkTree(), whole: set.New[string](), lines: map[string]set.Set[int]{}}
	if staged {
		err = own.readIndex(repo)
	} else {
		var base gitread.OID
		if base, err = revision(repo, rev); err == nil {
			err = own.readWorkTree(repo, base, added, false)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("diff scope: %w", err)
	}
	return own, nil
}

// revision answers the commit a selector names. Git parses a spelling this
// package does not, such as a reflog entry.
func revision(repo *gitread.Repo, rev string) (gitread.OID, error) {
	oid, err := repo.Resolve(rev)
	if errors.Is(err, gitread.ErrUnknownRevision) {
		out, gitErr := gitIn(repo.WorkTree(), "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
		if gitErr != nil {
			return gitread.OID{}, gitErr
		}
		return gitread.ParseOID(strings.TrimSpace(out))
	}
	if err != nil {
		return gitread.OID{}, err
	}
	return repo.PeelCommit(oid)
}

// readIndex records each staged file that differs from HEAD, as git diff
// --cached names it. With no commit yet, every staged file is new.
func (o *Lines) readIndex(repo *gitread.Repo) error {
	idx, err := repo.Index()
	if err != nil {
		return err
	}
	headFiles := map[string]gitread.TreeEntry{}
	if head, err := repo.Head(); err == nil {
		if headFiles, err = repo.TreeNames(head); err != nil {
			return err
		}
	}
	var pairs []pair
	for _, e := range idx.Entries() {
		if e.Stage != 0 || e.IsGitlink() {
			continue
		}
		before, ok := headFiles[e.Path]
		if !ok {
			o.whole.Add(e.Path)
			continue
		}
		if before.OID == e.OID {
			continue
		}
		after, err := readBlobs(o.top, repo, []gitread.OID{e.OID})
		if err != nil {
			return err
		}
		pairs = append(pairs, pair{e.Path, before.OID, after[e.OID]})
	}
	return o.addPairs(repo, pairs, added)
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

// Claim records path as a file the fork wrote all of.
func (o *Lines) Claim(path string) {
	o.whole.Add(o.rel(path))
}

// Holds reports whether the fork wrote any line from first to last of path.
func (o *Lines) Holds(path string, first, last int) bool {
	return o.Scope(path).Holds(first, last)
}
