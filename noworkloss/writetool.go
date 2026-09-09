package noworkloss

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The Write tool's own refusals, which were a separate plugin until the

const recyclerTimeout = 3 * time.Second

// writeToolReason is the Write-specific half of editToolReason. Edit and
// NotebookEdit are not covered: they change a file that exists, which is the
// behaviour this rule is steering toward.
func writeToolReason(path string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Lstat(path); err == nil {
		return "blocked: " + path + " already exists, and Write replaces the whole file. Use Edit to change it."
	}
	if original, ok := inRecycleBin(path); ok {
		return "blocked: " + path + " is not missing -- it is in the recycle bin, and Write would author a fresh file over the top of it.\n" +
			"run: recycler restore " + original + "   # then use Edit"
	}
	return vacatedReason(path)
}

// vacatedReason answers the way around the two checks above: the refusal names
// a path, the path is emptied by hand, and the same Write goes again. A rename,
// an `rm` and a `git rm` all leave one shape behind. Git holds content at this
// path and the disk does not. So the answer follows that state, and no list of
// verbs decides it.
func vacatedReason(path string) string {
	root, rel, ok := trackedPath(path)
	if !ok {
		return ""
	}
	return "blocked: " + path + " is gone from the working tree, and git still holds " + rel + " at this path.\n" +
		"Write here authors a whole file over one that exists, with the refusal stepped around rather than answered.\n" +
		"run: git -C " + root + " restore -- " + rel + "   # then use Edit. Commit the removal first, if the file must really go."
}

// trackedPath reports the repository root and the path inside it, for a path
// git knows in the index or in HEAD. Every git failure falls through to allow.
func trackedPath(path string) (root, rel string, ok bool) {
	out, _, err := runGit(filepath.Dir(path), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", false
	}
	root = strings.TrimSpace(out)
	if root == "" {
		return "", "", false
	}
	rel, err = filepath.Rel(root, physicalPath(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false
	}
	// The index answers for a plain `rm` and a rename. HEAD answers for
	// `git rm`, which takes the entry out of the index as it goes.
	if out, _, err := runGit(root, "ls-files", "--", ":(literal)"+rel); err == nil && strings.TrimSpace(out) != "" {
		return root, rel, true
	}
	if _, _, err := runGit(root, "cat-file", "-e", "HEAD:"+rel); err == nil {
		return root, rel, true
	}
	return "", "", false
}

// physicalPath resolves the parent directory, since git reports the physical
// path and a temporary directory is a symlink on macOS. The file itself is
// gone by the time this runs, so only the parent can be resolved.
func physicalPath(path string) string {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return path
	}
	return filepath.Join(parent, filepath.Base(path))
}

// inRecycleBin asks recycler, which already tracks each item's original
// location. Every failure falls through to allow.
func inRecycleBin(path string) (string, bool) {
	out, err := recyclerList()
	if err != nil {
		return "", false
	}
	var items []struct {
		OriginalPath string `json:"original_path"`
	}
	if json.Unmarshal(out, &items) != nil {
		return "", false
	}
	// recycler records the physically resolved path, so on macOS /tmp/x arrives
	physical := physicalPath(path)
	for _, item := range items {
		if item.OriginalPath == path || item.OriginalPath == physical {
			return item.OriginalPath, true
		}
	}
	return "", false
}

func recyclerList() ([]byte, error) {
	if _, err := exec.LookPath("recycler"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), recyclerTimeout)
	defer cancel()
	var out, errb strings.Builder
	cmd := exec.CommandContext(ctx, "recycler", "list", "--json")
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}
