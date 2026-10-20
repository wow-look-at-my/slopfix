package slopfix_test

import _ "embed"

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/goembed"
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// fixture is a file the rule set has something to say about, and the rules it
// must provoke.
type fixture struct {
	name    string
	path    string
	content string
	wants   []string
}

// privateWorkflowPath answers a workflow path in an org repository, made once.
var privateWorkflowPath = sync.OnceValue(func() string {
	dir, err := os.MkdirTemp("", "slopfix-private-")
	if err != nil {
		panic(err)
	}
	c, err := slopfix.Materialize(dir, slopfix.RuleCase{Name: "private", Origin: slopfix.PrivateRunnerFixture})
	if err != nil {
		panic(err)
	}
	return filepath.Join(c.Root, ".github", "workflows", "ci.yml")
})

// pinnedFetch reads from testdata, which this repository's own check skips.
func pinnedFetch() string {
	body, err := os.ReadFile(filepath.Join("pins", "testdata", "fetch.sh"))
	if err != nil {
		panic(err)
	}
	return string(body)
}

// roundTripFixtures carry at least a single instance of every rule
// CheckContent reports.
func roundTripFixtures() []fixture {
	return []fixture{
		{
			name:    "document",
			path:    "doc.md",
			content: roundTripFixturesMd + strings.Repeat("The runner reads each event from the stream and writes the result to the store. ", 25) + "\n",
			wants: []string{
				slopfix.IDHardWrap,
				slopfix.IDLongBlock,
				ste.IDContraction,
				ste.IDModal,
				ste.IDSemicolon,
				english.IDCommaNever,
				ste.IDCommaSplice,
				ste.IDSentenceCap,
				ste.IDStaleCount,
				ste.IDPostdeterminer,
			},
		},
		{
			name: "source",
			path: "src.go",
			content: "package demo\n\n" +
				"// Batch GET two of them.\nfunc Batch() {}\n\n" +
				"// Hold is the gate the caller waits on. It reads every file the session\n" +
				"// touched, weighs each one against the budget the caller named, and refuses\n" +
				"// the write when any of them carries a finding no rewrite answers.\n" +
				"func Hold() {}\n\n" +
				"// Wait blocks until the gate answers. It reads from the .gitmodules parser\n" +
				"// above, and it holds the one top directory the walk was handed, and it\n" +
				"// keeps every entry it saw on the way down so a later caller can ask again.\n" +
				"func Wait() {}\n\n" +
				"// A trailing note nobody attached to any code at all, documenting nothing.\n\n" +
				"// Close answers the gate, and the caller it answers for is the\nfunc Close() {}\n",
			wants: []string{
				slopfix.IDCommentNumber,
				"comments/length",
				"comments/tail",
			},
		},
		{
			name:    "workflow",
			path:    ".github/workflows/ci.yml",
			content: roundTripFixturesYml,
			wants: []string{
				"yaml/test-in-workflow",
				"yaml/comment-block",
				"yaml/all-builds-job",
				"yaml/neutered-gate",
				"yaml/env-indirection",
				"yaml/script-expression",
				"yaml/push-tags",
				"yaml/org-action-ref",
				"yaml/concurrency",
			},
		},
		{
			name:    "broken workflow",
			path:    ".github/workflows/ci.yml",
			content: roundTripFixturesYml2,
			wants: []string{
				"yaml/run-script-syntax",
				"yaml/duplicate-step-key",
				"yaml/id-token",
			},
		},
		{
			name:    "token in a git url",
			path:    ".github/workflows/ci.yml",
			content: roundTripFixturesYml3,
			wants:   []string{"yaml/token-in-git-url"},
		},
		{
			name: "warnings",
			path: "warn.md",
			content: "The file is read by the gate.\n\n" +
				"The gate has read the file.\n\n" +
				"The gate file system cache lookup stopped.\n\n" +
				"The tool gives additional output.\n\n" +
				"Read the file from the disk and write the result to the store for the caller before the next build of the tree starts.\n\n" +
				"The gate reads the file. The gate writes the result. The gate waits for the caller. The gate stops the loop. " +
				"The gate starts the build. The gate ends the run. The gate fails the check.\n",
			wants: []string{
				ste.IDPassive,
				ste.IDTense,
				ste.IDNounCluster,
				ste.IDDictionary,
				ste.IDInstructionLength,
				ste.IDParagraphLength,
			},
		},
		{
			name:    "private workflow",
			path:    privateWorkflowPath(),
			content: roundTripFixturesYml4,
			wants: []string{
				"yaml/private-runner",
				"yaml/concurrency",
			},
		},
		{
			name:    "script",
			path:    "fetch.sh",
			content: pinnedFetch(),
			wants:   []string{pins.ID},
		},
		{
			name:    "go",
			path:    "tool.go",
			content: "package tool\n\nconst setup = `#!/bin/sh\nset -e\nif [ -z \"$HOME\" ]; then exit 1; fi\necho ok\n`\n",
			wants:   []string{goembed.ID},
		},
	}
}

// Every rule slopfix REPORTS, slopfix REPAIRS. A rule the fixer cannot answer
// leaves the reader ways out, and the org allows neither: hand-edit the
// prose, or delete the file.
//
// So the property is the whole contract: fix, then check, and find nothing.
func TestFixLeavesNoFindingBehind(t *testing.T) {
	for _, f := range roundTripFixtures() {
		t.Run(f.name, func(t *testing.T) {
			before := findingIDs(slopfix.CheckContent(f.path, f.content))
			for _, want := range f.wants {
				assert.Contains(t, before, want,
					"the fixture stopped provoking %s, so this case proves nothing about it", want)
			}

			repair := slopfix.Fix(slopfix.Request{Content: f.content, Path: f.path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
			after := slopfix.CheckContent(f.path, repair.Text)
			assert.Empty(t, quoted(after), "a repaired file still carries findings:\n%s", repair.Text)
			assert.Empty(t, repair.Findings, "the repair reported what it did not repair")
			assert.Empty(t, repair.Kept, "the repair left a hit for a reader to answer by hand")
		})
	}
}

// The same property through the file path a caller runs: `slopfix fix`
// rewrites the file, and `slopfix check` on what it wrote reports nothing.
func TestFixingAFileLeavesNothingForCheckToReport(t *testing.T) {
	for _, f := range roundTripFixtures() {
		t.Run(f.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), filepath.Base(f.path))
			require.NoError(t, os.WriteFile(path, []byte(f.content), 0o644))

			repair, err := slopfix.FixFileWith(path, slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
			require.NoError(t, err)
			require.True(t, repair.Changed)

			after, err := slopfix.CheckFile(path)
			require.NoError(t, err)
			assert.Empty(t, quoted(after))
		})
	}
}

// A number said in words is longer than the number. The comment rewrite can
// put a block back over the budget the length cut had brought it under. The
// cut runs before the rewrite, so the file came out of a single pass still
// carrying the finding. The caller had to know to run fix again.
func TestALengthenedNumberIsCutBackInTheSamePass(t *testing.T) {
	// The comment fits its budget as written and does not a single time every
	// number in it is said in words. This is the whole of the interaction.
	src := "package demo\n\n" +
		"// Reserve takes one slot out of the arena, hands the caller back one handle to it and then publishes the newest entry it has just made now.\n" +
		"func Reserve() {}\n"
	require.NotContains(t, findingIDs(slopfix.CheckContent("demo.go", src)), "comments/length",
		"the fixture has to start inside the budget, or it proves nothing about the rewrite")

	repair := slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	require.True(t, repair.Changed)
	assert.Empty(t, quoted(slopfix.CheckContent("demo.go", repair.Text)), repair.Text)
}

// Another pass changes nothing. A repair that provokes its own rule on the
// next run leaves a caller looping, and a hook applying it never settles.
func TestFixIsSettledAfterOnePass(t *testing.T) {
	for _, f := range roundTripFixtures() {
		t.Run(f.name, func(t *testing.T) {
			once := slopfix.Fix(slopfix.Request{Content: f.content, Path: f.path, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
			twice := slopfix.Fix(slopfix.Request{Content: once, Path: f.path}).Text
			assert.Equal(t, once, twice)
		})
	}
}

// The fixtures cover the rule set. Without this the case above passes by
// testing a shrinking share of the rules.
func TestEveryRuleAppearsInAFixture(t *testing.T) {
	covered := set.New[string]()
	for _, f := range roundTripFixtures() {
		covered.AddRange(f.wants...)
	}
	var missing []string
	for id := range slopfix.AllIDs().All() {
		if !covered.Contains(id) {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "no round-trip fixture provokes: %s", strings.Join(missing, ", "))
}

func findingIDs(findings []ste.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ID)
	}
	return out
}

// quoted renders the findings, so a break names what is left rather than a
// count. A repair answers a warning too, so a warning left over is a break.
func quoted(findings []ste.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.String())
	}
	return out
}

//go:embed testdata/round-trip-fixtures.md
var roundTripFixturesMd string

//go:embed testdata/round-trip-fixtures.yml
var roundTripFixturesYml string

//go:embed testdata/round-trip-fixtures-2.yml
var roundTripFixturesYml2 string

//go:embed testdata/round-trip-fixtures-3.yml
var roundTripFixturesYml3 string

//go:embed testdata/round-trip-fixtures-4.yml
var roundTripFixturesYml4 string
