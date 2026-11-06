// IDBinary is a tracked file that starts with the magic number of an executable.

package slopfix

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/gitread"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/walktree"
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
// it. The blob is zero for a tree that no repository holds.
type trackedFile struct {
	rel  string
	blob gitread.OID
}

// trackedFiles names each regular file git tracks under root, relative to it.
// A directory with no repository is read from disk, without a .git directory.
// A repository whose index cannot be read is an error. A disk walk there would
// read every ignored build output as a committed file.
func trackedFiles(root string) (*gitread.Repo, []trackedFile, error) {
	repo, err := gitread.OpenWorkTree(root)
	if err != nil {
		return nil, nil, fmt.Errorf("read the repository at %s: %w", root, err)
	}
	if repo != nil {
		idx, err := repo.Index()
		if err != nil {
			return nil, nil, fmt.Errorf("read the index in %s: %w", root, err)
		}
		under, _ := repo.Rel(root)
		var files []trackedFile
		for _, entry := range idx.Entries() {
			// Neither a gitlink nor a symlink holds file bytes.
			if !strings.HasPrefix(entry.Mode, "100") {
				continue
			}
			rel := entry.Path
			if under != "." && under != "" {
				var ok bool
				if rel, ok = strings.CutPrefix(rel, under+"/"); !ok {
					continue
				}
			}
			files = append(files, trackedFile{filepath.FromSlash(rel), entry.OID})
		}
		return repo, files, nil
	}
	var files []trackedFile
	walktree.Get(root).Visit(nil, func(path string, e walktree.Entry) {
		if !e.IsRegular() {
			return
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			files = append(files, trackedFile{rel: rel})
		}
	})
	return nil, files, err
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
	defer trace.Phase("repo/binaries")()
	repo, files, err := trackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var out []TreeFinding
	var removed []string
	for _, file := range files {
		path := filepath.Join(root, file.rel)
		kind := fileKind(path)
		// The checkout is cheap to read, so only an executable there costs a blob read.
		if kind == "" || (repo != nil && blobKind(repo, file.blob) == "") {
			continue
		}
		if writing && writable(path) {
			if err := os.Remove(path); err != nil {
				return out, removed, err
			}
			walktree.Removed(path)
			removed = append(removed, path)
			continue
		}
		out = append(out, repoFinding(file.rel, IDBinary, "a committed "+kind+" executable",
			"Delete it and ignore it in .gitignore. A build makes it from source. `slopfix fix` deletes it."))
	}
	return out, removed, nil
}

// blobKind reads the first bytes of a blob in the object store.
func blobKind(repo *gitread.Repo, blob gitread.OID) string {
	data, ok := repo.Blob(blob)
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
