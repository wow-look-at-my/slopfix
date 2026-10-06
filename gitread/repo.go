package gitread

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Repo is an open git repository.
type Repo struct {
	workTree  string
	gitDir    string
	commonDir string

	mu     sync.Mutex
	objs   map[OID]object
	packs  []*pack
	idx    *Index
	cfg    map[string]string
	ignore *Ignore
	attrs  *Attributes
	extra  []*Repo
}

type object struct {
	typ  string
	data []byte
}

// Open finds the repository that holds dir, or nil when dir is in none.
func Open(dir string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("gitread: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	r, ok := discover(abs)
	if !ok {
		return nil, nil
	}
	return r, nil
}

// OpenWorkTree finds the repository whose work tree holds dir, or nil when the
// path is outside every work tree (a bare repository answers nil too).
func OpenWorkTree(dir string) (*Repo, error) {
	r, err := Open(dir)
	if err != nil || r == nil || r.workTree == "" {
		return nil, err
	}
	return r, nil
}

// WorkTree answers the resolved work tree root, or "" for a bare repository.
func (r *Repo) WorkTree() string { return r.workTree }

// GitDir answers the per-work-tree git directory.
func (r *Repo) GitDir() string { return r.gitDir }

// CommonDir answers the shared git directory.
func (r *Repo) CommonDir() string { return r.commonDir }

// discover walks up from dir to the repository that owns it.
func discover(dir string) (*Repo, bool) {
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() {
				return newRepo(dir, gitPath), true
			}
			if gd, ok := gitdirFile(gitPath); ok {
				return newRepo(dir, gd), true
			}
		}
		if isGitDir(dir) {
			return newRepo("", dir), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, false
		}
		dir = parent
	}
}

// gitdirFile reads the gitdir pointer a linked work tree and a submodule write.
func gitdirFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	rest, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if !filepath.IsAbs(rest) {
		rest = filepath.Join(filepath.Dir(path), rest)
	}
	abs, err := filepath.Abs(rest)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return abs, true
}

// isGitDir reports whether dir is itself a git directory.
func isGitDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "objects")); err != nil {
		return false
	}
	return true
}

func newRepo(workTree, gitDir string) *Repo {
	common := gitDir
	if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		dir := strings.TrimSpace(string(data))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(gitDir, dir)
		}
		if abs, err := filepath.Abs(dir); err == nil {
			common = filepath.Clean(abs)
		}
	}
	if workTree != "" {
		if resolved, err := filepath.EvalSymlinks(workTree); err == nil {
			workTree = resolved
		}
	}
	return &Repo{workTree: workTree, gitDir: gitDir, commonDir: common, objs: map[OID]object{}}
}

// IsShallow reports whether the repository is a shallow clone.
func (r *Repo) IsShallow() bool {
	_, err := os.Stat(filepath.Join(r.commonDir, "shallow"))
	return err == nil
}

// Path answers a path inside the per-work-tree git directory.
func (r *Repo) Path(parts ...string) string {
	return filepath.Join(append([]string{r.gitDir}, parts...)...)
}

// commonPath answers a path inside the shared git directory.
func (r *Repo) commonPath(parts ...string) string {
	return filepath.Join(append([]string{r.commonDir}, parts...)...)
}

// config answers the parsed repository config, cached.
func (r *Repo) config() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg != nil {
		return r.cfg
	}
	r.cfg = parseConfig(filepath.Join(r.commonDir, "config"))
	return r.cfg
}

// Config answers a configuration value, or "" when it is unset.
func (r *Repo) Config(key string) string { return r.config()[key] }

// OriginURL answers the URL the origin remote clones from, or "".
func (r *Repo) OriginURL() string { return r.Config("remote.origin.url") }

// parseConfig reads the INI subset a repository config uses.
func parseConfig(path string) map[string]string {
	out := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return out
	}
	defer file.Close()
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = sectionKey(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if i := strings.IndexAny(value, "#;"); i >= 0 && strings.TrimSpace(value[:i]) != "" {
			value = strings.TrimSpace(value[:i])
		}
		full := section
		if full != "" {
			full += "."
		}
		out[strings.ToLower(full+key)] = value
	}
	return out
}

// sectionKey turns a config section header into its dotted key.
func sectionKey(header string) string {
	header = strings.TrimSpace(header)
	if name, sub, ok := strings.Cut(header, " "); ok {
		name = strings.TrimSpace(name)
		sub = strings.Trim(strings.TrimSpace(sub), `"`)
		return name + "." + sub
	}
	return header
}

// Rel answers path relative to the work tree in slash form, or false when the
// path lies outside it.
func (r *Repo) Rel(path string) (string, bool) {
	if r.workTree == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(r.workTree, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// Resolve answers the object a revision names, following symbolic refs.
func (r *Repo) Resolve(rev string) (OID, error) {
	return r.resolve(rev, 0)
}

func (r *Repo) resolve(rev string, depth int) (OID, error) {
	if depth > 10 {
		return OID{}, fmt.Errorf("gitread: the ref %q does not resolve", rev)
	}
	if looksLikeOID(rev) {
		return ParseOID(rev)
	}
	data, ok := r.readRef(rev)
	if !ok {
		return OID{}, fmt.Errorf("gitread: the ref %q is not found", rev)
	}
	line := strings.TrimSpace(string(data))
	if target, isSym := strings.CutPrefix(line, "ref:"); isSym {
		return r.resolve(strings.TrimSpace(target), depth+1)
	}
	if !looksLikeOID(line) {
		return OID{}, fmt.Errorf("gitread: the ref %q names no object", rev)
	}
	return ParseOID(line)
}

// readRef answers the raw content of a ref file, loose first then packed.
func (r *Repo) readRef(name string) ([]byte, bool) {
	for _, base := range []string{r.gitDir, r.commonDir} {
		if data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(name))); err == nil {
			return data, true
		}
	}
	if oid, ok := r.packedRef(name); ok {
		return []byte(oid.String()), true
	}
	return nil, false
}

// Head answers the commit HEAD names.
func (r *Repo) Head() (OID, error) { return r.Resolve("HEAD") }

// packedRef answers a ref from the packed-refs file.
func (r *Repo) packedRef(name string) (OID, bool) {
	file, err := os.Open(filepath.Join(r.commonDir, "packed-refs"))
	if err != nil {
		return OID{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}
		oid, ref, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if ref == name {
			parsed, err := ParseOID(oid)
			if err != nil {
				return OID{}, false
			}
			return parsed, true
		}
	}
	return OID{}, false
}

// Refs answers every ref the packed-refs file and the loose refs name.
func (r *Repo) Refs(prefix string) map[string]OID {
	out := map[string]OID{}
	file, err := os.Open(filepath.Join(r.commonDir, "packed-refs"))
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || line[0] == '#' || line[0] == '^' {
				continue
			}
			oid, ref, ok := strings.Cut(line, " ")
			if !ok || !strings.HasPrefix(ref, prefix) {
				continue
			}
			if parsed, err := ParseOID(oid); err == nil {
				out[ref] = parsed
			}
		}
		file.Close()
	}
	root := filepath.Join(r.commonDir, filepath.FromSlash(prefix))
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.commonDir, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		line := strings.TrimSpace(string(data))
		if looksLikeOID(line) {
			if parsed, err := ParseOID(line); err == nil {
				out[filepath.ToSlash(rel)] = parsed
			}
		}
		return nil
	})
	return out
}

var errObjectMissing = errors.New("gitread: the object is not in this repository")
