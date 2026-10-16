package gitread

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrUnsupported marks a repository format this package does not read.
var ErrUnsupported = errors.New("gitread: the repository format is not supported")

// Repo is an open git repository. One Repo serves every caller that opens the
// same repository, so what one caller reads the next finds cached.
type Repo struct {
	workTree  string
	gitDir    string
	commonDir string
	indexFile string
	db        *odb

	// gitDirInfo and cfgStamp tell this repository from a new one at the same path.
	gitDirInfo os.FileInfo
	cfgStamp   fileStamp
	// reftable marks refs in the binary reftable format, which git reads for us.
	reftable bool

	cfgOnce sync.Once
	cfg     config

	mu         sync.Mutex
	idx        *Index
	idxStamp   fileStamp
	packed     map[string]OID
	packStamp  fileStamp
	shallow    map[OID]bool
	shallowStp fileStamp

	commits sync.Map
	rules   ruleCache
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
	workTree, gitDir, ok := locate(abs)
	if !ok {
		return nil, nil
	}
	return shared(workTree, gitDir)
}

// OpenWorkTree finds the repository whose work tree holds dir, or nil.
func OpenWorkTree(dir string) (*Repo, error) {
	r, err := Open(dir)
	if err != nil || r == nil || r.workTree == "" {
		return nil, err
	}
	return r, nil
}

// TopLevel answers the work tree root that holds path, a file or a directory.
func TopLevel(path string) (string, bool) {
	dir := path
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	r, err := OpenWorkTree(dir)
	if err != nil || r == nil {
		return "", false
	}
	return r.workTree, true
}

// WorkTree answers the resolved work tree root, or "" for a bare repository.
func (r *Repo) WorkTree() string { return r.workTree }

// GitDir answers the per-work-tree git directory.
func (r *Repo) GitDir() string { return r.gitDir }

// CommonDir answers the shared git directory.
func (r *Repo) CommonDir() string { return r.commonDir }

// repos holds every repository opened in this process.
var repos sync.Map

// shared answers the cached repository for a location, or opens it. A cached
// entry whose git directory was replaced is opened again.
func shared(workTree, gitDir string) (*Repo, error) {
	info, err := os.Stat(gitDir)
	if err != nil {
		return nil, nil
	}
	indexFile := os.Getenv("GIT_INDEX_FILE")
	if indexFile != "" && !filepath.IsAbs(indexFile) && workTree != "" {
		indexFile = filepath.Join(workTree, indexFile)
	}
	key := workTree + "\x00" + gitDir + "\x00" + indexFile
	if v, ok := repos.Load(key); ok {
		r := v.(*Repo)
		if os.SameFile(r.gitDirInfo, info) && stampOf(filepath.Join(r.commonDir, "config")) == r.cfgStamp {
			return r, nil
		}
	}
	r := newRepo(workTree, gitDir, indexFile, info)
	if err := r.supported(); err != nil {
		return nil, err
	}
	repos.Store(key, r)
	return r, nil
}

func newRepo(workTree, gitDir, indexFile string, info os.FileInfo) *Repo {
	common := gitDir
	if env := os.Getenv("GIT_COMMON_DIR"); env != "" {
		common = env
	} else if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		dir := strings.TrimSpace(string(data))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(gitDir, dir)
		}
		common = filepath.Clean(dir)
	}
	if indexFile == "" {
		indexFile = filepath.Join(gitDir, "index")
	}
	objects := filepath.Join(common, "objects")
	if env := os.Getenv("GIT_OBJECT_DIRECTORY"); env != "" {
		objects = env
	}
	return &Repo{workTree: workTree, gitDir: gitDir, commonDir: common, indexFile: indexFile, db: sharedODB(objects), gitDirInfo: info,
		cfgStamp: stampOf(filepath.Join(common, "config"))}
}

// supported refuses a format whose objects or refs live where this package
// does not look. Reading it would answer from the wrong place.
func (r *Repo) supported() error {
	if f := strings.ToLower(r.Config("extensions.objectformat")); f != "" && f != "sha1" {
		return fmt.Errorf("%w: %s uses the %s object format", ErrUnsupported, r.gitDir, f)
	}
	switch s := strings.ToLower(r.Config("extensions.refstorage")); s {
	case "", "files":
	case "reftable":
		r.reftable = true
	default:
		return fmt.Errorf("%w: %s keeps its refs in %s", ErrUnsupported, r.gitDir, s)
	}
	return nil
}

// locate answers the work tree and git directory for dir, as git finds them:
// GIT_DIR and GIT_WORK_TREE first, then a walk up to a .git entry.
func locate(dir string) (workTree, gitDir string, ok bool) {
	if env := os.Getenv("GIT_DIR"); env != "" {
		gitDir = absFrom(dir, env)
		if tree := os.Getenv("GIT_WORK_TREE"); tree != "" {
			return resolved(absFrom(dir, tree)), resolved(gitDir), true
		}
		if isBare(gitDir) {
			return "", resolved(gitDir), true
		}
		return dir, resolved(gitDir), true
	}
	for {
		dotGit := filepath.Join(dir, ".git")
		if info, err := os.Stat(dotGit); err == nil {
			if info.IsDir() && isGitDir(dotGit) {
				return dir, dotGit, true
			}
			if info.Mode().IsRegular() {
				if target, ok := gitdirFile(dotGit); ok {
					return dir, target, true
				}
			}
		}
		if filepath.Base(dir) != ".git" && isGitDir(dir) {
			return "", dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

func absFrom(dir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}

func resolved(path string) string {
	if out, err := filepath.EvalSymlinks(path); err == nil {
		return out
	}
	return path
}

// isBare reports a git directory whose config says it has no work tree.
func isBare(gitDir string) bool {
	cfg := readConfigFile(filepath.Join(gitDir, "config"), 0)
	return cfg["core.bare"] == "true"
}

// gitdirFile reads the pointer a linked work tree and a submodule write.
func gitdirFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if !filepath.IsAbs(rest) {
		rest = filepath.Join(filepath.Dir(path), rest)
	}
	return resolved(filepath.Clean(rest)), true
}

// isGitDir reports whether dir is itself a git directory.
func isGitDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "commondir")); err == nil {
		return true
	}
	_, err := os.Stat(filepath.Join(dir, "objects"))
	return err == nil
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
	abs = resolvedPrefix(abs)
	rel, err := filepath.Rel(r.workTree, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// resolvedPrefix resolves the links of the part of path that exists, so a
// path that does not exist yet is placed by its nearest directory.
func resolvedPrefix(path string) string {
	head, tail := path, ""
	for {
		if out, err := filepath.EvalSymlinks(head); err == nil {
			return filepath.Join(out, tail)
		}
		parent := filepath.Dir(head)
		if parent == head {
			return path
		}
		tail = filepath.Join(filepath.Base(head), tail)
		head = parent
	}
}
