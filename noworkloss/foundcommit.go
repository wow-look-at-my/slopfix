package noworkloss

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// foundPushTimeout bounds a found-work push.
const foundPushTimeout = 45 * time.Second

// foundResult is what committing and pushing one repository's found work did.
type foundResult struct {
	root   string
	branch string
	commit string
	paths  []string
	pushed bool
	// failure says why the work is not on a remote, and run is what finishes the job.
	failure string
	run     string
}

// foundWork is what a repository holds that only this machine has.
type foundWork struct {
	paths    []string
	ahead    int
	conflict bool
}

func (w foundWork) any() bool { return len(w.paths) > 0 || w.ahead > 0 || w.conflict }

// probeFound reads a repository's found work: every changed or untracked path
// git does not ignore, and the commits no remote-tracking ref contains.
func probeFound(root string) (foundWork, error) {
	var w foundWork
	out, _, err := runGit(root, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return w, err
	}
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		rec := fields[i]
		if len(rec) < 3 {
			continue
		}
		before := map[byte]int{'1': 8, '2': 9, 'u': 10, '?': 1}[rec[0]]
		if before == 0 {
			continue
		}
		parts := strings.SplitN(rec, " ", before+1)
		if len(parts) != before+1 {
			continue
		}
		if rec[0] == 'u' {
			w.conflict = true
		}
		if rec[0] == '2' {
			i++
		}
		w.paths = append(w.paths, parts[before])
	}
	if count, _, err := runGit(root, "rev-list", "--count", "HEAD", "--not", "--remotes"); err == nil {
		w.ahead, _ = strconv.Atoi(strings.TrimSpace(count))
	}
	return w, nil
}

// submodules answers the path of every submodule .gitmodules declares under root.
func submodules(root string) []string {
	out, _, err := runGit(root, "config", "-f", ".gitmodules", "--get-regexp", `\.path$`)
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			paths = append(paths, f[1])
		}
	}
	return paths
}

// commitFound commits and pushes the found work of root. A submodule goes
// first, so the parent's commit records the pushed gitlink. It answers a result
// for every repository that held found work.
func commitFound(root, session string, depth int) []foundResult {
	var out []foundResult
	if depth < maxScriptDepth {
		for _, sub := range submodules(root) {
			dir := filepath.Join(root, sub)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				out = append(out, commitFound(dir, session, depth+1)...)
			}
		}
	}
	work, err := probeFound(root)
	if err != nil {
		return append(out, foundResult{root: root, failure: "git status failed: " + err.Error(), run: "git -C " + shellQuote(root) + " status"})
	}
	if !work.any() {
		return out
	}
	return append(out, commitOne(root, session, work))
}

// sessionBranch names the branch found work moves to off the default branch.
func sessionBranch(session string) string {
	name := regexp.MustCompile(`[^A-Za-z0-9_-]`).ReplaceAllString(session, "")
	if len(name) > 12 {
		name = name[:12]
	}
	if name == "" {
		name = "session"
	}
	return "no-work-loss/" + name
}

// defaultBranch answers the branch the remote's HEAD names, or "".
func defaultBranch(root string) string {
	out, _, err := runGit(root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "origin/")
}

// commitOne commits and pushes one repository's found work.
func commitOne(root, session string, work foundWork) foundResult {
	res := foundResult{root: root, paths: work.paths}
	q := shellQuote(root)
	message := foundMessage(work.paths)
	finish := func(steps ...string) string {
		var parts []string
		for _, s := range steps {
			parts = append(parts, "git -C "+q+" "+s)
		}
		return strings.Join(parts, " && ")
	}
	if work.conflict {
		res.failure = "a merge or a rebase is in progress, and its conflicts are unresolved"
		res.run = finish("status") + "   # resolve each conflict, commit, then push"
		return res
	}
	if _, _, err := runGit(root, "remote", "get-url", "origin"); err != nil {
		res.failure = "the repository has no origin remote"
		res.run = finish("remote add origin <url>", "add -A", "commit -m "+shellQuote(message), "push -u origin HEAD")
		return res
	}
	branch := strings.TrimSpace(firstOut(runGit(root, "symbolic-ref", "--short", "-q", "HEAD")))
	if branch == "" || branch == defaultBranch(root) || defaultBranch(root) == "" && (branch == "master" || branch == "main") {
		// The default branch takes no direct push, so the work moves to a branch named for the session.
		name := sessionBranch(session)
		if _, stderr, err := runGit(root, "switch", "-c", name); err != nil {
			if _, _, err2 := runGit(root, "switch", name); err2 != nil {
				res.failure = "cannot create the branch " + name + ": " + strings.TrimSpace(stderr)
				res.run = finish("switch -c "+shellQuote(name), "add -A", "commit -m "+shellQuote(message), "push -u origin HEAD")
				return res
			}
		}
		branch = name
	}
	res.branch = branch
	if len(work.paths) > 0 {
		commit, ok := commitPaths(root, work.paths, func(bool) string { return message })
		if !ok && hasDiff(root) {
			res.failure = "the commit failed"
			res.run = finish("add -A", "commit -m "+shellQuote(message), "push -u origin HEAD")
			return res
		}
		res.commit = commit
	}
	args := []string{"push", "origin", "HEAD"}
	if _, _, err := runGit(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err != nil {
		args = []string{"push", "-u", "origin", "HEAD"}
	}
	if _, stderr, err := runGitHooked(root, foundPushTimeout, args...); err != nil {
		res.failure = "the push was refused: " + strings.TrimSpace(stderr)
		res.run = finish(strings.Join(args, " "))
		return res
	}
	res.pushed = true
	return res
}

// hasDiff reports whether the working tree still differs from HEAD.
func hasDiff(root string) bool {
	out, _, err := runGit(root, "status", "--porcelain")
	return err != nil || strings.TrimSpace(out) != ""
}

func firstOut(stdout, _ string, _ error) string { return stdout }

func foundMessage(paths []string) string {
	if len(paths) == 0 {
		return "no-work-loss: push found work"
	}
	return fmt.Sprintf("no-work-loss: commit found work before it is set aside\n\n%s", strings.Join(paths, "\n"))
}

// runGitHooked runs git with the repository's own hooks, so a pre-push check
// still judges what the guard pushes.
func runGitHooked(dir string, timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")
	err = cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return out.String(), errb.String(), err
}

// report says what found-work commits did, one line each.
func (r foundResult) report() string {
	if r.failure != "" {
		return fmt.Sprintf("found work in %s is not on a remote: %s.\nrun: %s", r.root, r.failure, r.run)
	}
	what := "pushed its unpushed commits"
	if r.commit != "" {
		what = fmt.Sprintf("committed %s (%s) and pushed it", plural(len(r.paths), "path", "paths"), shortSHA(r.commit))
	}
	return fmt.Sprintf("found work in %s: %s on %s.", r.root, what, r.branch)
}
