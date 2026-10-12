package forkscope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Repo is the repository a work tree names on the GitHub server.
type Repo struct {
	// Name is OWNER/NAME. It is empty when the work tree names no repository there.
	Name string
	// Private is what GitHub says.
	Private bool
}

// Owner answers the owner part of the name.
func (r Repo) Owner() string {
	owner, _, _ := strings.Cut(r.Name, "/")
	return owner
}

// visibilityFile is the name, in the git common directory, of the visibility record.
const visibilityFile = "slopfix-visibility.json"

// visibilityRecord is what the network said about a repository's visibility.
type visibilityRecord struct {
	Repo    string    `json:"repo"`
	Source  string    `json:"source"`
	Checked time.Time `json:"checked"`
	Private bool      `json:"private"`
}

// visibilities remembers each repository's visibility for the life of the process.
var visibilities = struct {
	sync.Mutex
	byKey map[string]bool
}{byKey: map[string]bool{}}

// Visibility answers the repository of the work tree that holds root. The
// network is asked only for a repository of owner. A repository of another
// owner answers its name, and Private stays false. A visibility that cannot
// be read is an error.
func (r Resolver) Visibility(root, owner string) (Repo, error) {
	top, topErr := toplevel(root)
	repo := ""
	if topErr == nil {
		repo = r.originRepo(top)
	}
	if repo == "" && r.workspaceHolds(root) {
		repo = r.getenv("GITHUB_REPOSITORY")
	}
	out := Repo{Name: repo}
	if repo == "" || !strings.EqualFold(out.Owner(), owner) {
		return out, nil
	}
	key := r.api() + " " + strings.ToLower(repo)
	visibilities.Lock()
	private, ok := visibilities.byKey[key]
	visibilities.Unlock()
	if !ok {
		var err error
		if private, err = r.readVisibility(top, topErr, repo); err != nil {
			return out, err
		}
		visibilities.Lock()
		visibilities.byKey[key] = private
		visibilities.Unlock()
	}
	out.Private = private
	return out, nil
}

// readVisibility answers whether repo is private: from the Actions event
// payload, then the work tree's record, then the API.
func (r Resolver) readVisibility(top string, topErr error, repo string) (bool, error) {
	if private, ok := r.eventPrivate(repo); ok {
		return private, nil
	}
	if topErr == nil {
		if rec, ok := r.cachedVisibility(top, repo); ok {
			return rec.Private, nil
		}
	}
	info, err := r.fetchRepo(repo)
	if err != nil {
		return false, err
	}
	if info.Private == nil {
		return false, fmt.Errorf("fork scope: the API answered %s with no private flag", repo)
	}
	if topErr == nil {
		rec := visibilityRecord{Repo: repo, Source: r.api(), Checked: time.Now(), Private: *info.Private}
		if err := writeRecord(top, visibilityFile, rec); err != nil {
			return false, err
		}
	}
	return *info.Private, nil
}

// eventPrivate answers the private flag the Actions event payload at
// GITHUB_EVENT_PATH gives repo, and whether it gives one.
func (r Resolver) eventPrivate(repo string) (bool, bool) {
	path := r.getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return false, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	var event struct {
		Repository *struct {
			FullName string `json:"full_name"`
			Private  *bool  `json:"private"`
		} `json:"repository"`
	}
	if json.Unmarshal(data, &event) != nil || event.Repository == nil || event.Repository.Private == nil {
		return false, false
	}
	if !strings.EqualFold(event.Repository.FullName, repo) {
		return false, false
	}
	return *event.Repository.Private, true
}

// cachedVisibility answers the record in top's git common directory when it
// names repo, came from this resolver's API and is younger than CacheTTL.
func (r Resolver) cachedVisibility(top, repo string) (visibilityRecord, bool) {
	path, err := recordPath(top, visibilityFile)
	if err != nil {
		return visibilityRecord{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return visibilityRecord{}, false
	}
	var rec visibilityRecord
	if json.Unmarshal(data, &rec) != nil {
		return visibilityRecord{}, false
	}
	age := time.Since(rec.Checked)
	if !strings.EqualFold(rec.Repo, repo) || rec.Source != r.api() || age > CacheTTL || age < 0 {
		return visibilityRecord{}, false
	}
	return rec, true
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
