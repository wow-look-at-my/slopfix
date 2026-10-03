package slopfix

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// IDBinary is a tracked file that starts with the magic number of an executable.
const IDBinary = "repo/binary"

// executableMagic holds the opening bytes of ELF, Mach-O and PE/COFF files.
var executableMagic = []struct {
	name  string
	bytes []byte
}{
	{"ELF", []byte{0x7f, 'E', 'L', 'F'}},
	{"Mach-O", []byte{0xfe, 0xed, 0xfa, 0xce}},
	{"Mach-O", []byte{0xfe, 0xed, 0xfa, 0xcf}},
	{"Mach-O", []byte{0xcf, 0xfa, 0xed, 0xfe}},
	{"Mach-O", []byte{0xce, 0xfa, 0xed, 0xfe}},
	{"Mach-O universal", []byte{0xca, 0xfe, 0xba, 0xbe}},
	{"PE/COFF", []byte{'M', 'Z'}},
}

// trackedFiles names each file git tracks under root, relative to it. A tree
// that git cannot list is read from disk, without its .git directory.
func trackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		var files []string
		for _, name := range strings.Split(string(out), "\x00") {
			if name != "" {
				files = append(files, filepath.FromSlash(name))
			}
		}
		return files, nil
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// executableKind names the executable format that head opens with, or answers "".
func executableKind(head []byte) string {
	for _, magic := range executableMagic {
		if bytes.HasPrefix(head, magic.bytes) {
			return magic.name
		}
	}
	return ""
}

// committedBinaries reports each tracked executable under root. Writing, it
// deletes them, because an image builds from source. Git still holds each.
func committedBinaries(root string, writing bool) ([]TreeFinding, []string, error) {
	files, err := trackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	embedded := embeddedFiles(root, files)
	var out []TreeFinding
	var removed []string
	for _, rel := range files {
		path := filepath.Join(root, rel)
		kind := fileKind(path)
		if kind == "" || embedded.Contains(filepath.ToSlash(rel)) {
			continue
		}
		if writing {
			if err := os.Remove(path); err != nil {
				return out, removed, err
			}
			removed = append(removed, path)
			continue
		}
		out = append(out, repoFinding(rel, IDBinary, "a committed "+kind+" executable",
			"Delete it and ignore it in .gitignore. A build makes it from source. `slopfix fix` deletes it."))
	}
	return out, removed, nil
}

// embeddedFiles names, in slash form relative to root, each file that a //go:embed directive in a tracked Go file names.
// The program carries such a file as an input, so no build makes it.
func embeddedFiles(root string, files []string) set.Set[string] {
	out := set.New[string]()
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		dir := filepath.Join(root, filepath.Dir(rel))
		for line := range strings.SplitSeq(string(body), "\n") {
			patterns, ok := strings.CutPrefix(strings.TrimSpace(line), "//go:embed ")
			if !ok {
				continue
			}
			for _, pattern := range strings.Fields(patterns) {
				matches, _ := filepath.Glob(filepath.Join(dir, filepath.FromSlash(strings.Trim(pattern, "\"`"))))
				for _, match := range matches {
					addEmbedded(out, root, match)
				}
			}
		}
	}
	return out
}

// addEmbedded adds path, or every file under it when it is a directory.
func addEmbedded(out set.Set[string], root, path string) {
	_ = filepath.WalkDir(path, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if rel, err := filepath.Rel(root, file); err == nil {
			out.Add(filepath.ToSlash(rel))
		}
		return nil
	})
}

// fileKind reads the first bytes of a regular file. A symlink, a gitlink and a
// path the checkout lacks answer "".
func fileKind(path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	return executableKind(head[:n])
}
