package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A comment that runs longer than its declaration is the whole rule.
func TestALongCommentOverAShortDeclarationIsFound(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The base URL every container's GitHub traffic rides. This is a constant",
		"// rather than a knob, because routing through the mirror is unconditional,",
		"// and an off switch would silently un-cache the whole fleet the first time",
		"// somebody reached for it in a hurry.",
		`const base = "https://example.invalid"`,
	}, "\n")

	hits := Check("x.go", src)
	require.Len(t, hits, 1)
	assert.Equal(t, ID, hits[0].ID)
	assert.Equal(t, 3, hits[0].Line)
	assert.Contains(t, hits[0].Tell, "longer than the code")
	assert.True(t, hits[0].Repairable)
}

// The control that proves the case above can fail: a comment in proportion to
// its code is not a finding.
func TestAProportionateCommentIsClean(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Never derive this: the server owns the URL grammar.",
		"func serve(mux *http.ServeMux) {",
		"\tmux.Handle(\"/\", index())",
		"\tmux.Handle(\"/health\", health())",
		"\tmux.Handle(\"/metrics\", metrics())",
		"}",
	}, "\n")
	assert.Empty(t, Check("x.go", src))
}

// A short note over a short line must never be a finding, whatever the ratio.
// Without the floor every useful sentence in the tree becomes one.
func TestAShortCommentIsAlwaysAllowed(t *testing.T) {
	src := "package p\n\n// The port.\nconst p = 1\n"
	assert.Empty(t, Check("x.go", src))
}

// The rule reads the source adapter, so it spans every language that adapter
// spells rather than only Go.
func TestTheRuleSpansLanguages(t *testing.T) {
	essay := []string{
		"// This helper exists because the caller cannot know the answer, and the",
		"// answer changes per platform, and the platform is decided at run time by",
		"// something none of this code owns, which is why it is a function at all",
		"// rather than a constant somebody could read at a glance.",
	}
	hash := []string{
		"# This helper exists because the caller cannot know the answer, and the",
		"# answer changes per platform, and the platform is decided at run time by",
		"# something none of this code owns, which is why it is a function at all",
		"# rather than a constant somebody could read at a glance.",
	}

	for name, tc := range map[string]struct {
		file string
		src  string
	}{
		"c":      {"x.c", strings.Join(append(essay, "int n = 1;"), "\n")},
		"cpp":    {"x.cpp", strings.Join(append(essay, "int n = 1;"), "\n")},
		"rust":   {"x.rs", strings.Join(append(essay, "let n = 1;"), "\n")},
		"bash":   {"x.sh", strings.Join(append(hash, "n=1"), "\n")},
		"python": {"x.py", strings.Join(append(hash, "n = 1"), "\n")},
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, Check(tc.file, tc.src), "no finding for %s", tc.file)
		})
	}
}

// A span a line walk worked out is reported and never rewritten. Wrong by a
// line, a rewrite deletes the wrong sentence, and nobody reviews what a hook
// applied. Go is parsed, so Go repairs; the rest report until they are too.
func TestAGuessedSpanIsReportedButNotRewritten(t *testing.T) {
	src := strings.Join([]string{
		"# This helper exists because the caller cannot know the answer, and the",
		"# answer changes per platform, and the platform is decided at run time by",
		"# something none of this code owns, which is why it is a function at all",
		"# rather than a constant somebody could read at a glance.",
		"n=1",
	}, "\n")

	hits := Check("x.sh", src)
	require.NotEmpty(t, hits, "a guessed span still reports")
	assert.False(t, hits[0].Repairable)

	out, changed := Fix("x.sh", src)
	assert.False(t, changed, "a guessed span is never rewritten")
	assert.Equal(t, src, out)
}

// A language the adapter does not spell is skipped rather than guessed at.
func TestAnUnknownLanguageIsSkipped(t *testing.T) {
	assert.Empty(t, Check("x.unknownext", "// a very long comment about nothing at all whatsoever\nvalue"))
}

// A trailing comment on a code line documents nothing of its own, so it is
// never measured against the statement beside it.
func TestATrailingCommentIsNotABlock(t *testing.T) {
	src := "package p\n\nconst p = 1 // the port this listens on, chosen years ago for reasons nobody wrote down anywhere\n"
	assert.Empty(t, Check("x.go", src))
}

// The repair cuts from the end, and the opening sentence survives.
func TestFixCutsTheTrailingProse(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Never derive this: the server owns the URL grammar.",
		"//",
		"// It used to be built here, which is how every published preview link kept",
		"// naming the legacy spelling long after the canonical form moved, and the",
		"// fallback below is for a server older than the field that reports it.",
		`const base = "https://example.invalid"`,
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "Never derive this: the server owns the URL grammar.")
	assert.NotContains(t, out, "legacy spelling")
	assert.Contains(t, out, `const base = "https://example.invalid"`)
	assert.Empty(t, Check("x.go", out), "the repaired file is clean")
}

// A file with nothing to repair comes back byte-identical, so a formatter run
// over a clean tree is a no-op.
func TestFixLeavesACleanFileAlone(t *testing.T) {
	src := "package p\n\n// The port.\nconst p = 1\n"
	out, changed := Fix("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}

// A block whose opening sentence alone still exceeds the budget is left as it
// is and still reported. A repair that deletes the only sentence worth keeping
// is worse than the finding.
func TestAnUnrepairableBlockIsReportedAndUntouched(t *testing.T) {
	long := "// " + strings.Repeat("a very long single opening sentence that will not fit ", 6)
	src := "package p\n\n" + long + "\nconst p = 1\n"

	hits := Check("x.go", src)
	require.Len(t, hits, 1)
	assert.False(t, hits[0].Repairable)

	out, changed := Fix("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}

// A comment marker inside a string is data, and the adapter is what keeps it
// out of the walk.
func TestAMarkerInsideAStringIsNotAComment(t *testing.T) {
	src := "package p\n\nconst u = \"https://example.invalid // not a comment at all, just a URL with slashes\"\n"
	assert.Empty(t, Check("x.go", src))
}

// Fixing back to front keeps an earlier block's line numbers valid, so several
// findings in one file all repair.
func TestSeveralBlocksInAFileAllRepair(t *testing.T) {
	essay := "// The point.\n//\n// Then a paragraph of elaboration that runs well past the length of the\n// declaration it sits above, several lines of it, saying little.\n"
	src := "package p\n\n" + essay + "const a = 1\n\n" + essay + "const b = 2\n"

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Empty(t, Check("x.go", out))
	assert.Contains(t, out, "const a = 1")
	assert.Contains(t, out, "const b = 2")
	assert.Equal(t, 2, strings.Count(out, "// The point."))
}
