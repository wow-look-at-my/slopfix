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
	// Sentences with a dash aside, the shapes that no division may halve and every one must still leave under the cap.
	files["README.md"] += "\nThere is no way to declare additional writable paths: something to write is the temp directory \u2014 a real filesystem inside every backend \u2014 and a command that genuinely needs the host is not a sandboxed command, so it belongs to a run without the sandbox.\n" +
		"\nEvery spawned workload command \u2014 test instances and setup hooks \u2014 runs at low priority (nice 19 applied to the command's process group) so a heavily parallel run does not starve the machine.\n" +
		"\n**Reads are confined under the sandbox**: a command sees the OS tool tree, the working directory, and the paths the file declared \u2014 not the home directory, not the temp root, not another checkout on the machine.\n" +
		"\nThe scrubber redacts sensitive names (material parameter names, asset names, and the paths built from them) in cooked output by **same-byte-length hash replacement** -- either on the loose files between cook and staging, or directly inside an existing archive (see the archive mode below).\n"
	files["README.md"] += "\nOutput is buffered and printed in canonical order \u2014 files in the order given on the command line (or discovered), instances in expansion order within each file \u2014 regardless of completion order.\n" +
		"\nEach re-run executes the **complete original argument scope**, never a subset. The runner has no test filtering or selection by design \u2014 every instance always runs \u2014 and `watch` adds no narrowing flags.\n" +
		"\n- Every **directory argument** (and the current directory in no-arg mode), recursively \u2014 with the same hidden-directory skip rules as discovery \u2014 so newly created `.cfg` files and new subdirectories are picked up and join the scope.\n" +
		"\nA reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a careful repair rather than a quiet deletion.\n"
	// A one-line JSDoc and a banner title each end a thought that a length cut keeps.
	files["page.ts"] = "import path from 'node:path';\n\n/** Absolute path of the exporter under test. */\n// The path.resolve call (not `new URL(..., import.meta.url)`) avoids the shadowed URL global of the jsdom test environment.\n// Node's fileURLToPath rejects cross-realm URLs.\n// A file path avoids both problems.\nexport const EXPORTER_MODULE = path.resolve(\n\timport.meta.dirname,\n\t'../../src/exporter.user.ts',\n);\n"
	files["steer.ts"] = "const FALLBACK_UNDER = 0.75;\n// ---- continuous wall-clock steer (hybrid signal) ----------------------------\n// Per-pass timestamps cannot see the overhead between passes, so the steer reads the wall clock between presents and folds it into the model each frame.\nconst STEER_GAIN = 0.2;\n"
	// A block over the volume cap above code longer than the cap, so the length cut alone leaves it over. A separate paragraph states a count, so comments/number is exercised too.
	files["main.go"] = "package main\n\n// There are 3 modes today.\n\n" + strings.Join(comment, "\n") + "\nfunc sum() int {\n\ttotal := 0\n" + strings.Join(body, "\n") + "\n\treturn total\n}\n\nfunc main() { sum() }\n"
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

// A fix over a tree of every rule's hardest case leaves no error, and no
// finding asks for a rewrite by hand.
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
		"pins/download-version", tombstones.IDVolume, "comments/length", "comments/number",
	} {
		assert.True(t, before.Contains(id), "the corpus holds no case of %s", id)
	}

	fixed := slopfix.FixTreeWith(root, req)
	assert.Empty(t, fixed.Unmet)
	assert.Empty(t, errorLines(slopfix.CheckTreeWith(root, req)), "a fix left these errors")

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "warns at many lines and errors at a higher count", "every count in the clause reads the same way")
	assert.Contains(t, string(readme), "for the team. The cache in the main store holds", "a long sentence divides where both halves are sentences")
	assert.Contains(t, string(readme), "near the wooden barn. That barn is behind the tall green hills", "a sentence with no verb restates the noun a phrase of place describes")
	assert.NotContains(t, string(readme), "This is", "no repair writes filler")
	assert.Contains(t, string(readme), "for the build (the cache) and for the test (the tree).", "a semicolon between pairs becomes a list of pairs")
	assert.Contains(t, string(readme), "`code span here`", "a code span stays whole")
	assert.Contains(t, string(readme), "\"a long quoted phrase with many words\"", "a quotation stays whole")
	assert.FileExists(t, filepath.Join(root, "justfile"))
	assert.FileExists(t, filepath.Join(root, "docs", "agents-continued.md"))
}
