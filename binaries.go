package slopfix

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/gitread"
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

// trackedFile is a regular file git tracks, and the blob the index holds for
// it. The blob is empty for a tree that git cannot list.
type trackedFile struct {
	rel  string
	blob string
}

// trackedFiles names each regular file git tracks under root, relative to it.
// A tree outside a work tree is read from disk, without its .git directory.
func trackedFiles(root string) ([]trackedFile, error) {
	repo, err := gitread.OpenWorkTree(root)
	if err == nil && repo != nil {
		if idx, err := repo.Index(); err == nil {
			var files []trackedFile
			for _, entry := range idx.Entries() {
				// Neither a gitlink nor a symlink holds file bytes.
				if entry.Stage != 0 || !strings.HasPrefix(entry.Mode, "100") {
					continue
				}
				abs := filepath.Join(repo.WorkTree(), filepath.FromSlash(entry.Path))
				rel, err := filepath.Rel(root, abs)
				if err != nil {
					continue
				}
				files = append(files, trackedFile{rel: rel, blob: entry.OID.String()})
			}
			return files, nil
		}
	}
	var files []trackedFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
			files = append(files, trackedFile{rel: rel})
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

// committedBinaries reports each executable that git itself stores under root.
// Writing, it deletes them, because an image builds from source. Git still
// holds each. A Git LFS file stores a pointer in git, so it never matches. A
// binary writable refuses stays a finding.
func committedBinaries(root string, writing bool, writable func(string) bool) ([]TreeFinding, []string, error) {
	files, err := trackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var out []TreeFinding
	var removed []string
	for _, file := range files {
		path := filepath.Join(root, file.rel)
		kind := fileKind(path)
		// The checkout is cheap to read, so only an executable there costs a blob read.
		if kind == "" || (file.blob != "" && blobKind(root, file.blob) == "") {
			continue
		}
		if writing && writable(path) {
			if err := os.Remove(path); err != nil {
				return out, removed, err
			}
			removed = append(removed, path)
			continue
		}
		out = append(out, repoFinding(file.rel, IDBinary, "a committed "+kind+" executable",
			"Delete it and ignore it in .gitignore. A build makes it from source. `slopfix fix` deletes it."))
	}
	return out, removed, nil
}

// blobKind reads the first bytes of a blob in the object store.
func blobKind(root, blob string) string {
	repo, err := gitread.OpenWorkTree(root)
	if err != nil || repo == nil {
		return ""
	}
	oid, err := gitread.ParseOID(blob)
	if err != nil {
		return ""
	}
	data, ok := repo.Blob(oid)
	if !ok {
		return ""
	}
	return executableKind(data[:min(len(data), 4)])
}

// fileKind reads the first bytes of a regular file. A symlink and a path the
// checkout lacks answer "".
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
