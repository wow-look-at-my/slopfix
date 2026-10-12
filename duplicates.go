package slopfix

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/trace"
)

// IDNearDuplicate is a file that is nearly the same as another file of the same name.
const IDNearDuplicate = "repo/near-duplicate"

// NearDuplicateShare is the share of lines at which files of one name are the same file.
const NearDuplicateShare = 0.95

// perDirectoryNames are files a tool wants in each directory it reads, so some
// of them alike is normal.
var perDirectoryNames = set.Of(
	"package.json", "package-lock.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", "yarn.lock", "tsconfig.json", "ts0.json", "justfile",
	"go.mod", "go.sum", "Cargo.toml", "Cargo.lock", "pyproject.toml",
	"Dockerfile", ".gitignore", ".gitattributes", ".dockerignore", ".editorconfig",
	"LICENSE", ClaudeFile, AgentsFile,
)

// DuplicateBlock is a range of lines in the later file, counting from one.
type DuplicateBlock struct{ Start, End int }

// DuplicateReport is one near-duplicate pair, with the portions of the later
// file that also appear in the earlier one.
type DuplicateReport struct {
	PathA, PathB string
	Share        float64
	Portions     []DuplicateBlock
}

// nearDuplicateReports answers every pair of files with one name that share
// lines above the threshold, and the portions of the later file to cut.
func nearDuplicateReports(root string) ([]DuplicateReport, error) {
	defer trace.Phase("repo/near-duplicates")()
	byName := map[string][]string{}
	for _, path := range commentfix.TreeFilesMatching(root, func(path string) bool {
		return !perDirectoryNames.Contains(filepath.Base(path))
	}) {
		// A symlink is the other file, not a copy of it.
		if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := filepath.Base(path)
		byName[name] = append(byName[name], path)
	}
	var out []DuplicateReport
	for _, paths := range byName {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		type read struct {
			path  string
			lines []string
		}
		var files []read
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				return out, err
			}
			if bytes.IndexByte(content, 0) >= 0 {
				continue
			}
			files = append(files, read{path: path, lines: strings.Split(string(content), "\n")})
		}
		// Each copy is reported once, against the first path it matches.
		for i, b := range files {
			for _, a := range files[:i] {
				// A file with no text holds nothing to share.
				if len(nonBlank(a.lines)) == 0 || len(nonBlank(b.lines)) == 0 {
					continue
				}
				share := lineShare(nonBlank(a.lines), nonBlank(b.lines))
				if share < NearDuplicateShare {
					continue
				}
				out = append(out, DuplicateReport{
					PathA:    a.path,
					PathB:    b.path,
					Share:    share,
					Portions: duplicatedPortions(a.lines, b.lines),
				})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PathB < out[j].PathB })
	return out, nil
}

// duplicatedPortions answers the ranges of the later file to delete: each top-level function or class. That duplicatedPortions is with the comment run above it, whose every line also appears in the earlier file. Only
// the rest of the file stays.
func duplicatedPortions(a, b []string) []DuplicateBlock {
	have := map[string]int{}
	for _, line := range a {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			have[trimmed]++
		}
	}
	blocks := topLevelBlocks(b)
	if len(blocks) == 0 {
		return sharedRuns(have, b)
	}
	var out []DuplicateBlock
	for _, block := range blocks {
		if blockShared(have, b, block) {
			out = append(out, block)
		}
	}
	return out
}

// blockShared reports whether every line of a block also appears in the other
// file, so the block is one of that file's.
func blockShared(have map[string]int, b []string, block DuplicateBlock) bool {
	seen := map[string]int{}
	for i := block.Start - 1; i < block.End && i < len(b); i++ {
		trimmed := strings.TrimSpace(b[i])
		if trimmed == "" {
			continue
		}
		seen[trimmed]++
		if seen[trimmed] > have[trimmed] {
			return false
		}
	}
	return true
}

// sharedRuns answers the runs of lines that appear in the other file.
func sharedRuns(have map[string]int, b []string) []DuplicateBlock {
	seen := map[string]int{}
	shared := make([]bool, len(b))
	for i, line := range b {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if seen[trimmed] < have[trimmed] {
			seen[trimmed]++
			shared[i] = true
		}
	}
	var out []DuplicateBlock
	for i := 0; i < len(shared); i++ {
		if !shared[i] {
			continue
		}
		start := i
		for i < len(shared) && shared[i] {
			i++
		}
		out = append(out, DuplicateBlock{Start: start + 1, End: i})
	}
	return out
}

// topLevelBlocks answers the top-level blocks of a source file: a declaration
// and the body it opens, back to the comment run directly above it. It reads
// the braces, because a function or a class is what a brace at the outermost
// level opens.
func topLevelBlocks(lines []string) []DuplicateBlock {
	var out []DuplicateBlock
	depth := 0
	open := 0
	for i, line := range lines {
		for _, r := range line {
			switch r {
			case '{', '(':
				if depth == 0 && open == 0 {
					open = i
				}
				depth++
			case '}', ')':
				if depth > 0 {
					depth--
				}
				if depth == 0 && open > 0 {
					out = append(out, DuplicateBlock{Start: commentStart(lines, open) + 1, End: i + 1})
					open = 0
				}
			}
		}
	}
	return out
}

// commentStart moves a block's opening line back over the comment run directly
// above it, so the block carries its own documentation.
func commentStart(lines []string, at int) int {
	start := at
	for start > 0 {
		above := strings.TrimSpace(lines[start-1])
		if strings.HasPrefix(above, "//") || strings.HasPrefix(above, "#") ||
			strings.HasPrefix(above, "/*") || strings.HasPrefix(above, "*") {
			start--
			continue
		}
		break
	}
	return start
}

// removePortions deletes the line ranges from a file, from the last backward so
// an earlier range stays valid.
func removePortions(path string, portions []DuplicateBlock) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for i := len(portions) - 1; i >= 0; i-- {
		p := portions[i]
		if p.Start < 1 || p.End > len(lines) || p.Start > p.End {
			continue
		}
		lines = append(lines[:p.Start-1], lines[p.End:]...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// nearDuplicates reports each near-duplicate as a finding, with the portions a
// repair cuts.
func nearDuplicates(root string) ([]TreeFinding, []DuplicateReport, error) {
	reports, err := nearDuplicateReports(root)
	if err != nil {
		return nil, nil, err
	}
	var out []TreeFinding
	for _, r := range reports {
		relA, _ := filepath.Rel(root, r.PathA)
		relB, _ := filepath.Rel(root, r.PathB)
		out = append(out, repoFinding(relB, IDNearDuplicate,
			fmt.Sprintf("%.1f%% of its lines match %s", r.Share*100, filepath.ToSlash(relA)),
			"Delete the duplicated functions or classes from this copy. `slopfix fix` does this."))
	}
	return out, reports, nil
}

// nonBlank answers the trimmed lines of s that hold text.
func nonBlank(s []string) []string {
	var out []string
	for _, line := range s {
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
