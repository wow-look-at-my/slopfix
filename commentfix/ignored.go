package commentfix

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// withoutIgnored drops each path git ignores, such as built output. Git never
// ignores a tracked file, so every tracked file stays. Outside a work tree git
// fails, and then every path stays.
func withoutIgnored(root string, paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	abs := make([]string, len(paths))
	for i, path := range paths {
		a, err := filepath.Abs(path)
		if err != nil {
			return paths
		}
		abs[i] = a
	}
	cmd := exec.Command("git", "check-ignore", "--stdin", "-z")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(abs, "\x00") + "\x00")
	out, err := cmd.Output()
	// Exit status 1 means git ignores none of the paths.
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return paths
	}
	ignored := set.Of(strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")...)
	var kept []string
	for i, path := range paths {
		if !ignored.Contains(abs[i]) {
			kept = append(kept, path)
		}
	}
	return kept
}

// withoutVendored drops each path .gitattributes marks with any of
// tombstones.BorrowedAttributes. Such text has another author, and no rule
// reads or rewrites it. Outside a work tree git fails, and then every path stays.
func withoutVendored(root string, paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	abs := make([]string, len(paths))
	for i, path := range paths {
		a, err := filepath.Abs(path)
		if err != nil {
			return paths
		}
		abs[i] = a
	}
	cmd := exec.Command("git", append([]string{"check-attr", "--stdin", "-z"}, tombstones.BorrowedAttributes...)...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(abs, "\x00") + "\x00")
	out, err := cmd.Output()
	if err != nil {
		return paths
	}
	vendored := set.New[string]()
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		if tombstones.AttributeSet(fields[i+2]) {
			vendored.Add(fields[i])
		}
	}
	var kept []string
	for i, path := range paths {
		if !vendored.Contains(abs[i]) {
			kept = append(kept, path)
		}
	}
	return kept
}
