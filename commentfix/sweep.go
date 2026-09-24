// sweep.go is the whole-tree face of the rule: which files carry prose a rule
// reads, which directories hold text nobody here authored, and how a repaired
// file is written back. A caller names the root and reads the result.
//
// The walk lives here rather than in each caller. A caller that owns its own
// walk owns its own skip list too, and skip lists drift: any of them ends up
// rewriting a vendored tree or a submodule, which belongs to somebody else.
package commentfix

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// skipDirs hold text nobody in the tree authored.
var skipDirs = set.Of("vendor", "node_modules", "testdata", "build")

// MaxFileBytes is where a file stops being prose and becomes a blob.
const MaxFileBytes = 1 << 20

// TreeResult is what a whole-tree run did.
type TreeResult struct {
	// Read is how many files the walk opened.
	Read int
	// Repaired names each file the sweep rewrote.
	Repaired []string
	// Removed quotes what the repair took, and is the only record of it.
	Removed []Removal
	// Findings carry what no repair covered, and the repair covers everything.
	Findings []Finding
}

// Removal is a piece of prose the repair deleted.
type Removal struct {
	Path string
	Text string
}

// Finding is a number that survived the repair.
type Finding struct {
	Path   string
	Line   int
	Col    int
	Number string
}

// FixTree rewrites every comment under root and reports what it did. A file is
// written only when the repair changed it.
func FixTree(root string) TreeResult {
	var out TreeResult
	for _, path := range TreeFiles(root) {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out.Read++
		fixed := Fix(path, string(src))
		if fixed.Changed && write(path, fixed.Text) == nil {
			out.Repaired = append(out.Repaired, path)
		}
		for _, text := range fixed.Removed {
			out.Removed = append(out.Removed, Removal{Path: path, Text: text})
		}
		out.Findings = append(out.Findings, findings(path, fixed.Text)...)
	}
	return out
}

// CheckTree reads the same files and writes none, so a caller can ask what a
// repair would find without taking the repair.
func CheckTree(root string) TreeResult {
	var out TreeResult
	for _, path := range TreeFiles(root) {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out.Read++
		out.Findings = append(out.Findings, findings(path, string(src))...)
	}
	return out
}

// findings keeps a finding per line rather than per number, because the repair
// is a rewrite of the line whatever it counts.
func findings(path, src string) []Finding {
	seen := set.New[int]()
	var out []Finding
	for _, hit := range Check(path, src) {
		if seen.Contains(hit.Line) {
			continue
		}
		seen.Add(hit.Line)
		out = append(out, Finding{Path: path, Line: hit.Line, Col: hit.Col, Number: hit.Number})
	}
	return out
}

// write renames a temp file over the target, so a concurrent reader never sees a torn file.
func write(path, text string) error {
	info, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".slopfix")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// TreeFiles returns every file under root whose comments this rule reads.
func TreeFiles(root string) []string {
	return TreeFilesMatching(root, Supported)
}

// TreeFilesMatching is TreeFiles for a caller whose rule reads a different set
// of files. The walk, and so the skip list, stays what this package owns.
func TreeFilesMatching(root string, reads func(string) bool) []string {
	// Where the root is not a module, the modules below it are the whole tree.
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	rootIsModule := err == nil
	var out []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(root, path, d.Name(), rootIsModule) {
				return filepath.SkipDir
			}
			return nil
		}
		if !reads(path) {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Size() > MaxFileBytes {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out
}

// skipDir reports whether the walk stops here. A nested module's prose belongs
// to that module, and a submodule's to another repository: no fix here lands in
// either of them.
func skipDir(root, path, name string, rootIsModule bool) bool {
	if path == root {
		return false
	}
	if name == ".github" {
		// The workflows the gate reads live under a dotted directory, so the rule.
		return false
	}
	if strings.HasPrefix(name, ".") || skipDirs.Contains(name) {
		return true
	}
	if IsSubmodule(path) {
		return true
	}
	return rootIsModule && isNestedModule(path)
}

// IsSubmodule reports whether dir is a git submodule's working tree. Git marks
// such a tree by writing .git as a FILE holding a gitdir pointer.
func IsSubmodule(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.Mode().IsRegular()
}

// isNestedModule reports whether dir declares a module of its own.
func isNestedModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
