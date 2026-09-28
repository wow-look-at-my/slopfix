package noworkloss

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The Write tool's own refusals.

const recyclerTimeout = 3 * time.Second

// writeToolReason is the Write-specific half of editToolReason. Edit and
// NotebookEdit are not covered: they change a file that exists, which is the
// behaviour this rule is steering toward.
func writeToolReason(path string, w writeAttempt) string {
	if path == "" {
		return ""
	}
	if _, err := os.Lstat(path); err == nil {
		return "blocked: " + path + " already exists, and Write replaces the whole file. Use Edit to change it."
	}
	if item, ok := inRecycleBin(path); ok {
		return binnedReason(path, item)
	}
	return vacatedReason(path, w)
}

// binnedReason puts a recycled file back before it answers, as vacatedReason
// does for git. Several Writes go out in one batch. A refusal that only names
// the restore then reaches each of them, and none of them can act on it.
func binnedReason(path string, item binItem) string {
	if err := item.restore(); err != nil {
		return "blocked: " + path + " is not missing -- it is in the recycle bin, and Write would author a fresh file over the top of it.\n" +
			"Putting it back failed (" + err.Error() + ").\n" +
			"run: recycler restore " + item.ID + "   # then use the Edit tool"
	}
	return "blocked: " + path + " was in the recycle bin. This hook put the file back.\n" +
		"No. Write does not replace a file that existed, however it was removed. Read it, then use the Edit tool."
}

// vacatedReason answers the way around the checks above: the refusal names a
// path, the path is emptied by hand, and the same Write goes again. A rename,
// an `rm`, a `git rm` and a committed deletion each leave the same shape
// behind. Git holds content at this path and the disk does not. So the answer
// follows that state, and no list of verbs decides it.
//
// A guard here mitigates rather than refuses, so the file goes back on disk
// before this answers. The Write then meets the ordinary refusal above, which
// is true again, and Edit has a file to work on. A restore that fails says
// what to run.
func vacatedReason(path string, w writeAttempt) string {
	held, ok := trackedPath(path)
	if !ok {
		return ""
	}
	if err := held.restore(); err != nil {
		return "blocked: " + path + " is gone from the working tree, and " + held.source + " still holds " + held.rel + " at this path.\n" +
			"Putting it back failed (" + err.Error() + "), so this Write authors a whole file over one that exists.\n" +
			"run: " + held.restoreCommand() + "   # then use the Edit tool\n" +
			w.wasted(path, held)
	}
	return "blocked: " + path + " was a file, and " + held.source + " holds its content. This hook put the file back.\n" +
		"No. Write does not replace a file that existed, however it was removed: deleting it, committing the deletion and writing it fresh is refused the same way. Use the Edit tool.\n" +
		w.wasted(path, held)
}

// heldPath is a path git holds and the disk does not, with the place git
// holds it: the index for a rename and a plain `rm`, HEAD for a `git rm`.
type heldPath struct {
	root, rel, source string
	// rev is the tree the restore reads, or "" for the index.
	rev string
}

// content is what the restore writes.
func (h heldPath) content() (string, error) {
	out, _, err := runGit(h.root, "show", h.rev+":"+h.rel)
	return out, err
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
	if h.rev != "" {
		// The index carries the entry, so the tree has to be named.
		args = append(args, "--source="+h.rev)
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
	if gitHas(root, "HEAD:"+rel) {
		return heldPath{root: root, rel: rel, source: "HEAD", rev: "HEAD"}, true
	}
	if commit, ok := recentDeletion(root, rel); ok {
		short := commit
		if len(short) > 12 {
			short = short[:12]
		}
		return heldPath{root: root, rel: rel, source: "the parent of " + short + ", the commit that deleted it,", rev: commit + "^"}, true
	}
	return heldPath{}, false
}

func gitHas(root, object string) bool {
	_, _, err := runGit(root, "cat-file", "-e", object)
	return err == nil
}

// recentDepth bounds how far back from HEAD a committed deletion is found.
const recentDepth = 10

// recentDeletion finds a commit in recent history that removed rel. A
// deletion committed only to free the path for Write is the same evasion as
// an `rm`, one step later. Recent is the branch's own commits, plus the last
// recentDepth, so a file deleted long ago on purpose can be created afresh.
func recentDeletion(root, rel string) (string, bool) {
	for _, window := range deletionWindows(root) {
		args := append([]string{"rev-list", "-1", "HEAD"}, window...)
		out, _, err := runGit(root, append(args, "--", ":(literal)"+rel)...)
		if err != nil {
			continue
		}
		commit := strings.TrimSpace(out)
		if commit == "" {
			continue
		}
		// HEAD lacks the path, so the last commit to touch it removed it.
		if gitHas(root, commit+":"+rel) || !gitHas(root, commit+"^:"+rel) {
			continue
		}
		return commit, true
	}
	return "", false
}

// deletionWindows answers the exclusions that bound each walk. A history
// shorter than recentDepth is walked whole.
func deletionWindows(root string) [][]string {
	var windows [][]string
	for _, base := range []string{"refs/remotes/origin/HEAD", "refs/remotes/origin/main", "refs/remotes/origin/master"} {
		if _, _, err := runGit(root, "rev-parse", "--verify", "-q", base); err == nil {
			windows = append(windows, []string{"--not", base})
			break
		}
	}
	floor := "HEAD~" + strconv.Itoa(recentDepth)
	if _, _, err := runGit(root, "rev-parse", "--verify", "-q", floor); err == nil {
		return append(windows, []string{"--not", floor})
	}
	return append(windows, nil)
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

// binItem is one entry of `recycler list --json`.
type binItem struct {
	ID           string    `json:"id"`
	OriginalPath string    `json:"original_path"`
	DeletedAt    time.Time `json:"deleted_at"`
}

// restore names the item by its ID. A path that was recycled more than once
// matches several items, and recycler refuses an ambiguous path.
func (b binItem) restore() error {
	_, err := runRecycler("restore", b.ID)
	return err
}

// inRecycleBin asks recycler, which already tracks each item's original
// location. It answers the newest item at the path. Every failure falls
// through to allow.
func inRecycleBin(path string) (binItem, bool) {
	out, err := runRecycler("list", "--json")
	if err != nil {
		return binItem{}, false
	}
	var items []binItem
	if json.Unmarshal([]byte(out), &items) != nil {
		return binItem{}, false
	}
	// recycler records the physically resolved path, so on macOS /tmp/x arrives
	physical := physicalPath(path)
	var newest binItem
	found := false
	for _, item := range items {
		if item.OriginalPath != path && item.OriginalPath != physical {
			continue
		}
		if !found || item.DeletedAt.After(newest.DeletedAt) {
			newest, found = item, true
		}
	}
	return newest, found
}

func runRecycler(args ...string) (string, error) {
	if _, err := exec.LookPath("recycler"); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), recyclerTimeout)
	defer cancel()
	var out, errb strings.Builder
	cmd := exec.CommandContext(ctx, "recycler", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errb.String()); msg != "" {
			return "", errors.New(firstLine(msg))
		}
		return "", err
	}
	return out.String(), nil
}
