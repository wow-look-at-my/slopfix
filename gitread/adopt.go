package gitread

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AdoptObjects copies every object file the other repository holds into
// this's store, so a deepened clone carries the history itself.
func (r *Repo) AdoptObjects(other *Repo) error {
	src := filepath.Join(other.CommonDir(), "objects")
	dst := filepath.Join(r.CommonDir(), "objects")
	if err := copyTree(src, dst); err != nil {
		return err
	}
	r.mu.Lock()
	r.packs = nil
	r.mu.Unlock()
	return nil
}

// copyTree copies every regular file under src into dst, keeping existing files.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(dst, rel)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Unshallow marks the repository complete by removing its shallow boundary.
func (r *Repo) Unshallow() error {
	err := os.Remove(filepath.Join(r.CommonDir(), "shallow"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("gitread: %w", err)
	}
	return nil
}

// isLocalRemote reports whether a remote URL names a path on this machine.
func isLocalRemote(remote string) bool {
	if strings.HasPrefix(remote, "file://") {
		return true
	}
	if strings.Contains(remote, "://") {
		return false
	}
	if strings.Contains(remote, "@") {
		return false
	}
	if strings.Contains(remote, ":") && !strings.HasPrefix(remote, "/") && !strings.HasPrefix(remote, ".") {
		return false
	}
	return true
}
