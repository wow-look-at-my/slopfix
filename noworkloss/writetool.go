package noworkloss

import (
	"context"
	"encoding/json"
	"errors"
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

// vacatedReason answers the way around the checks above: the refusal names a
// path, the path is emptied by hand, and the same Write goes again. A rename,
// an `rm` and a `git rm` each leave the same shape behind. Git holds content
// at this path and the disk does not. So the answer follows that state, and no
// list of verbs decides it.
//
// A guard here mitigates rather than refuses, so the file goes back on disk
// before this answers. The Write then meets the ordinary refusal above, which
// is true again, and Edit has a file to work on. A restore that fails says
// what to run.
func vacatedReason(path string) string {
	held, ok := trackedPath(path)
	if !ok {
		return ""
	}
	if err := held.restore(); err != nil {
		return "blocked: " + path + " is gone from the working tree, and " + held.source + " still holds " + held.rel + " at this path.\n" +
			"Putting it back failed (" + err.Error() + "), so this Write authors a whole file over one that exists.\n" +
			"run: " + held.restoreCommand() + "   # then use Edit"
	}
	return "blocked: " + path + " was gone from the working tree, and " + held.source + " still held " + held.rel + " at this path.\n" +
		"This hook put the file back. It exists again, so Write replaces content nobody read a diff of.\n" +
		"Use Edit to change it. Commit the removal first, if the file must really go."
}

// heldPath is a path git holds and the disk does not, with the place git holds
// it: the index for a rename and a plain `rm`, HEAD for a `git rm`.
type heldPath struct {
	root, rel, source string
}

// restore writes the path back into the working tree, and leaves the index
// alone. Nothing is destroyed: the caller establishes that no file sits at
// this path.
func (h heldPath) restore() error {
	_, stderr, err := runGit(h.root, append(h.restoreArgs(), "--", ":(literal)"+h.rel)...)
	if err != nil && strings.TrimSpace(stderr) != "" {
		return errors.New(strings.TrimSpace(firstLine(stderr)))
	}
	return err
}

func (h heldPath) restoreArgs() []string {
	args := []string{"restore", "--worktree"}
	if h.source == "HEAD" {
		// The index no longer carries the entry, so the tree has to be named.
		args = append(args, "--source=HEAD")
	}
	return args
}

func (h heldPath) restoreCommand() string {
	return "git -C " + h.root + " " + strings.Join(h.restoreArgs(), " ") + " -- " + h.rel
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// trackedPath reports where git holds a path the disk does not, and where it
// sits inside the repository. Every git failure falls through to allow.
func trackedPath(path string) (heldPath, bool) {
	out, _, err := runGit(filepath.Dir(path), "rev-parse", "--show-toplevel")
	if err != nil {
		return heldPath{}, false
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return heldPath{}, false
	}
	rel, err := filepath.Rel(root, physicalPath(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return heldPath{}, false
	}
	// The index answers for a plain `rm` and a rename. HEAD answers for
	// `git rm`, which takes the entry out of the index as it goes.
	if out, _, err := runGit(root, "ls-files", "--", ":(literal)"+rel); err == nil && strings.TrimSpace(out) != "" {
		return heldPath{root: root, rel: rel, source: "the index"}, true
	}
	if _, _, err := runGit(root, "cat-file", "-e", "HEAD:"+rel); err == nil {
		return heldPath{root: root, rel: rel, source: "HEAD"}, true
	}
	return heldPath{}, false
}

// physicalPath resolves the PARENT, because git reports a physical path and
// the file itself is already gone.
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
