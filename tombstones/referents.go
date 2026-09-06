// referents.go carries the tier that does not read the wording at all: a
// comment naming a symbol that exists nowhere is describing a tree that is
// gone. "see TestDarwinStatfsToLinux for the pin", written beside the change
// that deleted that test, is a tombstone with no tell in its phrasing, and no
// rewording makes it true again.
//
// Precision comes entirely from which identifiers are eligible. A comment about
// low-level work is full of names the repository does not define, and reporting
// those is how a guard earns the reputation that gets it turned off. So an
// all-caps name is never a candidate, and neither is a short one.
package tombstones

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// identifierWords splits text into the runs the shape test judges. Splitting on
// every character an identifier cannot contain yields exactly the runs a word
// boundary delimits.
func identifierWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !(r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9'))
	})
}

// minCandidate is the shortest name this rule will judge. Below it a run of
// letters is far more likely to be an English word than a symbol.
const minCandidate = 8

// isCandidate reports whether a name is one this repository must contain.
//
// An all-caps name is refused outright: ENOSYS and its kin are the shape a
// low-level comment is full of, and none of them is the repository's to define.
// A short name is refused from the other direction: it is too likely a word.
func isCandidate(name string) bool {
	return len(name) >= minCandidate &&
		name != strings.ToUpper(name) &&
		identifierShaped(name)
}

// identifierShaped reports whether a whole word is built like a symbol rather
// than prose: it carries an underscore, or a case change inside it.
func identifierShaped(name string) bool {
	if strings.Contains(name, "_") {
		return true
	}
	for i := 1; i < len(name); i++ {
		prev, c := name[i-1], name[i]
		if c >= 'A' && c <= 'Z' && (prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9') {
			return true
		}
	}
	// An initial capital is the remaining symbol shape, and prose capitalises a
	// sentence the same way. Only a name with no lower-case tail qualifies.
	return name[0] >= 'A' && name[0] <= 'Z' && strings.ContainsAny(name, "0123456789")
}

// probeTimeout bounds the search. A guard that hangs is worse than one that
// misses, so an expired probe reports nothing.
const probeTimeout = 2 * time.Second

// maxNames bounds how many identifiers one comment may put to the repository.
// An unbounded set is a comment this rule cannot judge cheaply.
const maxNames = 40

// DeadReferents returns the identifiers the blocks name that appear neither in
// the text being written nor anywhere in the repository holding path.
//
// It returns nothing at all when it cannot answer: no repository, no ripgrep, a
// timeout, or a search that errors. An absent answer must never read as "the
// symbol is gone".
func DeadReferents(path, added string, blocks []Block) []string {
	root := RepoRoot(path)
	if root == "" {
		return nil
	}
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil
	}

	names := map[string]bool{}
	for _, b := range blocks {
		for _, m := range identifierWords(b.Text) {
			if isCandidate(m) {
				names[m] = true
			}
		}
	}
	if len(names) == 0 || len(names) > maxNames {
		return nil
	}

	var ordered []string
	for n := range names {
		if strings.Count(added, n) > 1 {
			continue
		}
		ordered = append(ordered, n)
	}
	if len(ordered) == 0 {
		return nil
	}

	args := []string{"--no-messages", "--fixed-strings", "--files-with-matches", "--max-count", "1"}
	var dead []string
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	for _, name := range ordered {
		cmd := exec.CommandContext(ctx, rg, append(append([]string{}, args...), "-e", name, root)...)
		out, err := cmd.Output()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
			return nil // ripgrep failed rather than found nothing
		}
		if strings.TrimSpace(string(out)) == "" {
			dead = append(dead, name)
		}
	}
	return dead
}

// RepoRoot walks up from path looking for a working tree. An empty result means
// path is not inside one.
func RepoRoot(path string) string {
	dir := path
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
