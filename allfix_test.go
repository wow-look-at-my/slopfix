package slopfix_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// corpusFiles maps each fixture in testdata/allfix to the path it takes in the
// tree. A fixture keeps a name no rule reads, so no repair touches it in place.
var corpusFiles = map[string]string{
	"CLAUDE.md":    "CLAUDE.md",
	"README.md":    "README.md",
	"package.json": "package.json",
	"ci.yml.in":    ".github/workflows/ci.yml",
}

// hardCorpus holds the hardest case each error rule has, in one tree.
func hardCorpus(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for from, to := range corpusFiles {
		content, err := os.ReadFile(filepath.Join("testdata", "allfix", from))
		require.NoError(t, err)
		files[to] = string(content)
	}
	var agents []string
	for len(strings.Join(agents, "\n\n")) <= slopfix.CharBudget+2000 {
		agents = append(agents, "Each build reads the cache. The tool then writes the result to the store.")
	}
	var comment []string
	for i := range 5 {
		if i > 0 {
			comment = append(comment, "//")
		}
		comment = append(comment, "// The loop reads each value from the input and adds it to the total.",
			"// It then checks the total against the limit that the caller set.",
			"// A total over the limit stops the loop.")
	}
	var body []string
	for range 20 {
		body = append(body, "\ttotal++")
	}
	// The instruction file has no heading at all, so the budget repair has no section to move.
	files["AGENTS.md"] = strings.Join(agents, "\n\n") + "\n"
	// A list item over the block cap, with a parenthesis that runs past the division target.
	files["README.md"] += "\n- **Admin port**: (" + strings.Repeat("The gate reads the file. ", 50) + ") " + strings.Repeat("The tool writes the result to the store. ", 30) + "\n"
	// A block over the volume cap above code longer than the cap, so the length cut alone leaves it over.
	files["main.go"] = "package main\n\n" + strings.Join(comment, "\n") + "\nfunc sum() int {\n\ttotal := 0\n" + strings.Join(body, "\n") + "\n\treturn total\n}\n\nfunc main() { sum() }\n"
	return files
}

// errorLines answers every error a tree run reports, one line each.
func errorLines(out slopfix.TreeRepair) []string {
	var lines []string
	for _, f := range out.Findings {
		if !f.Warning() {
			lines = append(lines, f.Path+": "+f.Finding.String())
		}
	}
	for _, k := range out.Kept {
		lines = append(lines, k.Path+": "+k.ID+" "+k.Phrase)
	}
	sort.Strings(lines)
	return lines
}

// The owner's rule: slopfix must be able to repair everything, even if not
// well. A fix over a tree of every rule's hardest case leaves no error.
func TestFixLeavesNoErrorOnAnyTree(t *testing.T) {
	root := gitRoot(t, hardCorpus(t))
	req := slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines}

	before := set.New[string]()
	out := slopfix.CheckTreeWith(root, req)
	for _, f := range out.Findings {
		before.Add(f.ID)
	}
	for _, k := range out.Kept {
		before.Add(k.ID)
	}
	for _, id := range []string{
		"ste/sentence-length", "english/semicolon", "ste/contraction", "ste/modal", "wrap/hard-wrap", slopfix.IDLongBlock,
		slopfix.IDBudget, slopfix.IDPackageScripts, slopfix.IDAgentsFile,
		"yaml/push-tags", "yaml/concurrency", "yaml/all-builds-job", "yaml/neutered-gate", "yaml/comment-block", "yaml/org-action-ref",
		"pins/download-version", tombstones.IDVolume, "comments/length",
	} {
		assert.True(t, before.Contains(id), "the corpus holds no case of %s", id)
	}

	fixed := slopfix.FixTreeWith(root, req)
	assert.Empty(t, fixed.Unmet)
	assert.Empty(t, errorLines(slopfix.CheckTreeWith(root, req)), "a fix left these errors")

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "warns at many lines and errors at a higher count", "every count in the clause reads the same way")
	assert.Contains(t, string(readme), "`code span here`", "a code span stays whole")
	assert.Contains(t, string(readme), "\"a long quoted phrase with many words\"", "a quotation stays whole")
	assert.FileExists(t, filepath.Join(root, "justfile"))
	assert.FileExists(t, filepath.Join(root, "docs", "agents-continued.md"))
}
