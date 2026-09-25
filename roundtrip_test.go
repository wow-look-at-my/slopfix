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

// roundTripFixtures carry at least a single instance of every rule
// CheckContent reports.
func roundTripFixtures() []fixture {
	return []fixture{
		{
			name: "document",
			path: "doc.md",
			content: "# Title\n\n" +
				"A paragraph the author wrapped by hand\nacross two lines.\n\n" +
				"It doesn't run, and a caller should wait.\n\n" +
				"The gate is shut; the write fails.\n\n" +
				"The gate is shut, the write fails.\n\n" +
				"The gate reads every file in the session and the write fails when any one of them carries a finding that a rewrite cannot repair on its own.\n\n" +
				"The gate reads every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own.\n\n" +
				"The loader reads every row it holds into memory, which is the whole reason a caller waits on it before the header check runs.\n\n" +
				"Its four fields hold the header.\n\n" +
				"The set holds three rules.\n\n" +
				"It shares its substrate with two other rules, which claims nothing about what is here and is reported all the same.\n",
			wants: []string{
				slopfix.IDHardWrap,
				ste.IDContraction,
				ste.IDModal,
				ste.IDSemicolon,
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
			name: "workflow",
			path: ".github/workflows/ci.yml",
			content: "name: CI\n" +
				"on:\n" +
				"  push:\n" +
				"jobs:\n" +
				"  all-builds:\n" +
				"    runs-on: ubuntu-latest\n" +
				"    steps:\n" +
				"      # The gate runs here.\n" +
				"      # It reads the tree.\n" +
				"      - uses: wow-look-at-my/slopfix@v1\n" +
				"        continue-on-error: true\n" +
				"      - name: assert\n" +
				"        run: |\n" +
				"          grep -q ok out.txt || { echo \"::error::missing\"; exit 1; }\n",
			wants: []string{
				"yaml/comment-block",
				"yaml/all-builds-job",
				"yaml/test-in-workflow",
				"yaml/neutered-gate",
			},
		},
	}
}

// Every rule slopfix REPORTS, slopfix REPAIRS. A rule the fixer cannot answer
// leaves the reader ways out, and the org allows neither: hand-edit the
// prose, or delete the file. documents were deleted over exactly this.
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

// The same property through the file path a caller actually runs: `slopfix fix`
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

// A number said in words is longer than the number, so the comment rewrite can
// put a block back over the budget the length cut had just brought it under.
// The cut runs before the rewrite, so the file came out of a single pass still
// carrying the finding, and the caller had to know to run fix again.
func TestALengthenedNumberIsCutBackInTheSamePass(t *testing.T) {
	// The comment fits its budget as written and does not a single time every
	// number in it is said in words, which is the whole of the interaction.
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

// quoted renders findings for a failure message, so a break names what is left
// rather than a count.
func quoted(findings []ste.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.String())
	}
	return out
}
