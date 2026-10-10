package ratchet

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// File names, on its default branch, the command that judges a branch.
const File = ".github/ratchet"

// byParentVar is set in a run that reexecuted itself under the build it judges.
const byParentVar = "SLOPFIX_RATCHET_BY_PARENT"

// Check runs the default branch's ratchet against this branch.
func Check() error { return CheckIn("") }

// CheckIn runs the check in dir, or the process directory when dir is empty.
//
// The command the default branch's File names runs from a checkout of that
// branch. That command is with this branch's checkout as its last argument,
// and a non-zero exit fails. A run on the default branch itself passes,
// because that is where an owner's merge lands a change to the command.
//
// The check is a no-op outside CI and outside a repository, so a local run
// never reaches the network. A File that names no command is an error, never
// a pass.
func CheckIn(dir string) error {
	if os.Getenv("CI") == "" || os.Getenv(byParentVar) != "" {
		return nil
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return string(out), nil
	}

	// A directory outside any repository has no branch to compare.
	head, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	head = strings.TrimSpace(head)
	symref, err := git("ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return fmt.Errorf("ratchet: reading the default branch: %w", err)
	}
	branch := eachLsRemoteRef([]byte(symref), func(string, string) {})
	if branch == "" {
		return fmt.Errorf("ratchet: origin names no default branch:\n%s", symref)
	}
	if onDefaultBranch(branch) {
		return nil
	}
	if _, err := git("fetch", "--quiet", "--no-tags", "--depth=1", "origin", branch); err != nil {
		return fmt.Errorf("ratchet: fetching %s: %w", branch, err)
	}
	base, err := git("rev-parse", "FETCH_HEAD")
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)
	// A default branch that names no ratchet judges nothing.
	if _, err := git("cat-file", "-e", base+":"+File); err != nil {
		return nil
	}
	spec, err := git("show", base+":"+File)
	if err != nil {
		return err
	}
	argv := Command(spec)
	if len(argv) == 0 {
		return fmt.Errorf("ratchet: %s on %s names no command", File, branch)
	}

	checkout, err := os.MkdirTemp("", "ratchet-base")
	if err != nil {
		return err
	}
	defer os.RemoveAll(checkout)
	if _, err := git("worktree", "add", "--quiet", "--detach", checkout, base); err != nil {
		return fmt.Errorf("ratchet: checking out %s: %w", branch, err)
	}
	defer git("worktree", "remove", "--force", checkout)
	// A checkout holds no generated file, so the command generates first.
	if err := generateIn(checkout); err != nil {
		return err
	}

	cmd := exec.Command(argv[0], append(argv[1:], head)...)
	cmd.Dir = checkout
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ratchet: %s's %s (%s) fails on this branch: %w",
			branch, File, strings.Join(argv, " "), err)
	}
	return nil
}

// generateIn runs the generate directives of the checkout at dir, which the
// repository builds from and CI holds no generated file for. A directory with
// no go.mod generates nothing.
func generateIn(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return nil
	}
	back, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(back)
	if err := os.Chdir(dir); err != nil {
		return err
	}
	// A fresh checkout carries neither the generated files nor a go.sum that
	// resolves the branch-head modules, so both are settled first. GOFLAGS is
	// cleared because an explicit -mod flag turns off the toolchain's
	// resolution of a v0.0.0 org placeholder to its branch head.
	for _, args := range [][]string{{"mod", "tidy"}, {"generate", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ratchet: go %s in %s: %w", strings.Join(args, " "), dir, err)
		}
	}
	return nil
}

// onDefaultBranch reports whether this CI run is for the default branch.
// GitHub sets GITHUB_REF_NAME to the branch a push run is for.
func onDefaultBranch(branch string) bool {
	return os.Getenv("GITHUB_REF_NAME") == branch
}

// Command reads the command off the first line that is neither blank nor a
// comment.
func Command(spec string) []string {
	for line := range strings.SplitSeq(spec, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.Fields(line)
	}
	return nil
}

// eachLsRemoteRef walks an ls-remote answer's ref lines and answers the branch
// a symbolic HEAD resolves to.
func eachLsRemoteRef(out []byte, fn func(hash, ref string)) (branch string) {
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "ref: "); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				branch = strings.TrimPrefix(fields[0], "refs/heads/")
			}
			continue
		}
		switch fields := strings.Fields(line); len(fields) {
		case 0:
		case 1:
			fn(fields[0], "")
		default:
			fn(fields[0], fields[1])
		}
	}
	return branch
}
