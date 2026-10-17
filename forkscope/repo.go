package forkscope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RepoName answers the OWNER/NAME of the work tree that holds root.
func (r Resolver) RepoName(root string) string {
	top, topErr := toplevel(root)
	return r.repoName(root, top, topErr)
}

// repoName answers the repository that the origin of top names. With no such
// origin, it is GITHUB_REPOSITORY for a root inside GITHUB_WORKSPACE.
func (r Resolver) repoName(root, top string, topErr error) string {
	repo := ""
	if topErr == nil {
		if g, err := openRepo(top); err == nil {
			repo = r.originRepo(g)
		}
	}
	if repo == "" && r.workspaceHolds(root) {
		repo = r.getenv("GITHUB_REPOSITORY")
	}
	return repo
}

// RecordPlain records repo in the work tree at top as a repository that is no
// fork. A fixture states what the network would answer this way.
func (r Resolver) RecordPlain(top, repo string) error {
	g, err := openRepo(top)
	if err != nil {
		return err
	}
	return store(g, record{Repo: repo, Source: r.source(), Checked: time.Now(), Kind: kindPlain})
}

// ExistingDir answers dir, or its nearest parent that exists. A path that does
// not exist yet belongs to the work tree of that directory.
func ExistingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// writeRecord writes v as JSON into the file name of top's git common
// directory, through a rename.
func writeRecord(top, name string, v any) error {
	path, err := recordPath(top, name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), name+".*")
	if err != nil {
		return fmt.Errorf("fork scope: write %s: %w", name, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("fork scope: write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fork scope: write %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("fork scope: write %s: %w", name, err)
	}
	return nil
}
