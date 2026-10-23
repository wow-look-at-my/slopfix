// binarySniff is how far into a file git looks for a NUL byte.

package forkscope

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/gitread"
)

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
