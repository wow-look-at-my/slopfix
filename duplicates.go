package slopfix

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// IDNearDuplicate is a file that is nearly the same as another file of the same name.
const IDNearDuplicate = "repo/near-duplicate"

// NearDuplicateShare is the share of lines at which files of one name are the same file.
const NearDuplicateShare = 0.95

// CopyAttribute is the gitattribute that marks a file as a copy a contract requires.
const CopyAttribute = "slopfix-copy"

// perDirectoryNames are files a tool wants in each directory it reads, so some of them alike is normal.
var perDirectoryNames = set.Of(
	"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "tsconfig.json",
	"go.mod", "go.sum", "Cargo.toml", "Cargo.lock", "pyproject.toml",
	"Dockerfile", ".gitignore", ".gitattributes", ".dockerignore", ".editorconfig",
	"LICENSE", ClaudeFile, AgentsFile,
)

// nearDuplicates reports each file whose lines match another file of the same
// name at NearDuplicateShare or above. The later path of a pair is reported.
func nearDuplicates(root string) ([]TreeFinding, error) {
	byName := map[string][]string{}
	for _, path := range commentfix.TreeFilesMatching(root, func(path string) bool {
		return !perDirectoryNames.Contains(filepath.Base(path))
	}) {
		name := filepath.Base(path)
		byName[name] = append(byName[name], path)
	}
	var candidates []string
	for _, paths := range byName {
		if len(paths) > 1 {
			candidates = append(candidates, paths...)
		}
	}
	copies := withAttribute(root, candidates, CopyAttribute)
	var out []TreeFinding
	for _, paths := range byName {
		sort.Strings(paths)
		lines := map[string][]string{}
		for _, path := range paths {
			if copies.Contains(path) {
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return out, err
			}
			if bytes.IndexByte(content, 0) >= 0 {
				continue
			}
			if l := nonBlankLines(string(content)); len(l) > 0 {
				lines[path] = l
			}
		}
		for i, a := range paths {
			for _, b := range paths[i+1:] {
				if lines[a] == nil || lines[b] == nil {
					continue
				}
				share := lineShare(lines[a], lines[b])
				if share < NearDuplicateShare {
					continue
				}
				relA, _ := filepath.Rel(root, a)
				relB, _ := filepath.Rel(root, b)
				out = append(out, repoFinding(relB, IDNearDuplicate,
					fmt.Sprintf("%.1f%% of its lines match %s", share*100, filepath.ToSlash(relA)),
					"Keep one copy and use it from both places. Mark a copy with the "+CopyAttribute+
						" gitattribute only when a contract requires the copy."))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// nonBlankLines answers the trimmed lines of s that hold text.
func nonBlankLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// lineShare counts the lines files share, twice, over the lines of both. The
// share is symmetric, and a file much longer than the other scores low.
func lineShare(a, b []string) float64 {
	counts := map[string]int{}
	for _, line := range a {
		counts[line]++
	}
	shared := 0
	for _, line := range b {
		if counts[line] > 0 {
			shared++
			counts[line]--
		}
	}
	return float64(2*shared) / float64(len(a)+len(b))
}

// withAttribute names each path that git marks with attr. A tree git cannot
// read marks nothing.
func withAttribute(root string, paths []string, attr string) set.Set[string] {
	marked := set.New[string]()
	if len(paths) == 0 {
		return marked
	}
	abs := make([]string, len(paths))
	byAbs := map[string]string{}
	for i, path := range paths {
		a, err := filepath.Abs(path)
		if err != nil {
			return marked
		}
		abs[i] = a
		byAbs[a] = path
	}
	cmd := exec.Command("git", "check-attr", "--stdin", "-z", attr)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(abs, "\x00") + "\x00")
	out, err := cmd.Output()
	if err != nil {
		return marked
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		if value := fields[i+2]; value != "unspecified" && value != "unset" {
			if path, ok := byAbs[fields[i]]; ok {
				marked.Add(path)
			}
		}
	}
	return marked
}
