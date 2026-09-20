package commentfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// tree writes a repository the walk reads, and returns its root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}

// names answers the walk's result relative to the root, so a case reads as the
// paths it wrote.
func names(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, path := range commentfix.TreeFiles(root) {
		rel, err := filepath.Rel(root, path)
		require.NoError(t, err)
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func TestTheWalkSkipsTextNobodyHereAuthored(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod":              "module example.com/m\n",
		"main.go":             "package main\n",
		"docs/run.sh":         "#!/bin/sh\n",
		".hidden/x.go":        "package x\n",
		"vendor/v/v.go":       "package v\n",
		"node_modules/n/n.js": "const n = 1\n",
		"testdata/t.go":       "package t\n",
		"build/b.go":          "package b\n",
		"nested/go.mod":       "module example.com/n\n",
		"nested/n.go":         "package n\n",
		"README.md":           "# m\n",
	})
	assert.ElementsMatch(t, []string{"main.go", "docs/run.sh"}, names(t, root))
}

// A submodule's working tree belongs to another repository, which git marks by
// writing .git as a file.
func TestTheWalkSkipsASubmodule(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod":         "module example.com/m\n",
		"sub/.git":       "gitdir: ../.git/modules/sub\n",
		"sub/s.go":       "package s\n",
		"kept/k.go":      "package k\n",
		"kept/.git/HEAD": "ref: refs/heads/master\n",
	})
	assert.ElementsMatch(t, []string{"kept/k.go"}, names(t, root))
}

// Where the root declares no module of its own, the modules below it are the
// whole tree, so skipping them scans nothing at all.
func TestTheWalkKeepsTheModulesUnderANonModuleRoot(t *testing.T) {
	root := tree(t, map[string]string{
		"a/go.mod": "module example.com/a\n",
		"a/a.go":   "package a\n",
	})
	assert.ElementsMatch(t, []string{"a/a.go"}, names(t, root))
}

func TestTheWalkSkipsABlob(t *testing.T) {
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n"})
	big := make([]byte, commentfix.MaxFileBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "big.go"), big, 0o644))
	assert.Empty(t, names(t, root))
}

// The reason the rule reads a tree rather than a package: a shell script and a
// workflow carry the same stale prose a Go comment does, and no Go analyzer
// ever looked at either.
func TestTheWalkReadsEveryLanguageTheExtractorKnows(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod": "module example.com/m\n",
		"a.go":   "package p\n\n// the walk has 3 phases\n",
		"run.sh": "#!/bin/sh\n# the sweep runs twice\n",
		"ci.yml": "# holds 4 jobs\njobs: {}\n",
	})
	found := map[string]string{}
	for _, finding := range commentfix.CheckTree(root).Findings {
		found[filepath.Base(finding.Path)] = finding.Number
	}
	assert.Equal(t, map[string]string{"a.go": "3", "run.sh": "twice", "ci.yml": "4"}, found)
}

// A sentence naming several numbers costs a single finding. The repair is a
// rewrite of the line, whatever it counts.
func TestASentenceNamingSeveralNumbersIsOneFinding(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod": "module example.com/m\n",
		"a.go":   "package p\n\n// holds 5 entries across 3 shards\n",
	})
	assert.Len(t, commentfix.CheckTree(root).Findings, 1)
}

// The claim the caller relies on: a swept tree reports nothing.
func TestRepairLeavesNoFinding(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod":  "module example.com/m\n",
		"main.go": "package main\n\n// It reserves 4 slots and one lock.\nfunc main() {}\n",
	})
	result := commentfix.FixTree(root)
	assert.Empty(t, result.Findings)
	assert.Len(t, result.Repaired, 1)

	src, err := os.ReadFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(src), "func main() {}")
	assert.NotContains(t, string(src), "4 slots")
	assert.Empty(t, commentfix.CheckTree(root).Findings)
}

func TestRepairKeepsAFileItCannotImprove(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod":  "module example.com/m\n",
		"main.go": "package main\n\n// It reserves the slots.\nfunc main() {}\n",
	})
	result := commentfix.FixTree(root)
	assert.Empty(t, result.Repaired)
	assert.Equal(t, 1, result.Read)
}

// The read-only face answers what a repair would take, and takes nothing.
func TestReportWritesNothing(t *testing.T) {
	src := "package main\n\n// It reserves 4 slots.\nfunc main() {}\n"
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n", "main.go": src})
	assert.NotEmpty(t, commentfix.CheckTree(root).Findings)

	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	assert.Equal(t, src, string(after))
}

// The rename leaves no scratch file behind for the next walk to read.
func TestRepairLeavesNoTempFile(t *testing.T) {
	root := tree(t, map[string]string{
		"go.mod":  "module example.com/m\n",
		"main.go": "package main\n\n// It reserves 4 slots.\nfunc main() {}\n",
	})
	commentfix.FixTree(root)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), "slopfix")
	}
}
