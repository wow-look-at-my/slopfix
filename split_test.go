package slopfix

import _ "embed"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func para(word string, size int) string {
	return strings.TrimSpace(strings.Repeat(word+" ", size/(len(word)+1)))
}

func TestSplitMovesTheLargestSectionsUntilUnderTheTarget(t *testing.T) {
	big := para("big", 20_000)
	mid := para("mid", 15_000)
	small := para("small", 9_000)
	agents := testSplitMovesTheLargestSectionsUntilUnderTheTargetMd + big + testSplitMovesTheLargestSectionsUntilUnderTheTargetMd2 + mid + "\n\n## Small\n\n" + small + "\n"
	root := repo(t, map[string]string{"AGENTS.md": agents})

	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/big-one.md"}, written)

	got := readFile(t, root, "AGENTS.md")
	assert.LessOrEqual(t, len([]rune(got)), SplitTarget)
	assert.Contains(t, got, testSplitMovesTheLargestSectionsUntilUnderTheTargetMd3)
	assert.Contains(t, got, mid, "a section the target does not need stays")
	assert.Contains(t, got, "Preamble.")

	doc := readFile(t, root, filepath.FromSlash("docs/big-one.md"))
	assert.Equal(t, "# Big one\n\n"+big+"\n\n## Detail\n\nMore.\n", doc, "the text moves word for word, one heading level up")
}

func TestSplitLeavesAFileUnderTheTargetAlone(t *testing.T) {
	root := repo(t, map[string]string{"AGENTS.md": "## A\n\nshort\n"})
	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Empty(t, written)
	assert.Equal(t, "## A\n\nshort\n", readFile(t, root, "AGENTS.md"))
}

func TestSplitReadsNoHeadingInsideAFence(t *testing.T) {
	lines := strings.Split(testSplitReadsNoHeadingInsideAFenceMd, "\n")
	secs := sections(lines, 2)
	require.Len(t, secs, 2)
	assert.Equal(t, "Real", secs[0].title)
	assert.Equal(t, "Next", secs[1].title)
}

// A file with no level-2 heading splits at the headings it has.
func TestSplitMovesASectionOfAnyLevel(t *testing.T) {
	agents := "# Title\n\n### Deep\n\n" + para("deep", CharBudget) + "\n\n### Short\n\nkept\n"
	root := repo(t, map[string]string{"AGENTS.md": agents})
	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/deep.md"}, written)
	got := readFile(t, root, "AGENTS.md")
	assert.LessOrEqual(t, len([]rune(got)), SplitTarget)
	assert.Contains(t, got, testSplitMovesASectionOfAnyLevelMd)
}

// A cut inside a fence closes it, and the moved part opens it again.
func TestSplitClosesAFenceItCutsThrough(t *testing.T) {
	var code []string
	for range CharBudget / 20 {
		code = append(code, "echo line of code")
	}
	agents := "Intro.\n\n```sh\n" + strings.Join(code, "\n") + "\n```\n"
	root := repo(t, map[string]string{"AGENTS.md": agents})
	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/agents-continued.md"}, written)
	got := readFile(t, root, "AGENTS.md")
	assert.LessOrEqual(t, len([]rune(got)), SplitTarget)
	assert.Equal(t, 2, strings.Count(got, "```"), "the kept part closes its fence")
	assert.True(t, strings.HasSuffix(got, "```\n\n[docs/agents-continued.md](docs/agents-continued.md) holds the rest of this file.\n"), got[len(got)-200:])
	doc := readFile(t, root, filepath.FromSlash("docs/agents-continued.md"))
	assert.True(t, strings.HasPrefix(doc, testSplitClosesAFenceItCutsThroughMd), doc[:80])
	assert.Equal(t, len(code), strings.Count(got+doc, "echo line of code"), "no line is lost")
}

func TestSplitNeverOverwritesAnExistingDoc(t *testing.T) {
	root := repo(t, map[string]string{
		"AGENTS.md":     "## Topic\n\n" + para("x", CharBudget) + "\n",
		"docs/topic.md": "somebody's notes\n",
	})
	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/topic-2.md"}, written)
	assert.Equal(t, "somebody's notes\n", readFile(t, root, filepath.FromSlash("docs/topic.md")))
}

func TestSplitDryRunWritesNothing(t *testing.T) {
	agents := "## Topic\n\n" + para("x", CharBudget) + "\n"
	root := repo(t, map[string]string{"AGENTS.md": agents})
	written, err := Split(root, "AGENTS.md", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/topic.md"}, written)
	assert.Equal(t, agents, readFile(t, root, "AGENTS.md"))
	_, err = os.Stat(filepath.Join(root, "docs"))
	assert.True(t, os.IsNotExist(err))
}

func TestSlug(t *testing.T) {
	assert.Equal(t, "repo-the-markdown-a-repository-keeps", slug("repo: the markdown a repository keeps"))
	assert.Equal(t, "ask-properly-link-refs", slug("`ask-properly`, `link-refs`"))
	assert.Equal(t, "section", slug("!!!"))
}

//go:embed testdata/test-split-moves-the-largest-sections-until-under-the-target.md
var testSplitMovesTheLargestSectionsUntilUnderTheTargetMd string

//go:embed testdata/test-split-moves-the-largest-sections-until-under-the-target-2.md
var testSplitMovesTheLargestSectionsUntilUnderTheTargetMd2 string

//go:embed testdata/test-split-moves-the-largest-sections-until-under-the-target-3.md
var testSplitMovesTheLargestSectionsUntilUnderTheTargetMd3 string

//go:embed testdata/test-split-reads-no-heading-inside-a-fence.md
var testSplitReadsNoHeadingInsideAFenceMd string

//go:embed testdata/test-split-moves-a-section-of-any-level.md
var testSplitMovesASectionOfAnyLevelMd string

//go:embed testdata/test-split-closes-a-fence-it-cuts-through.md
var testSplitClosesAFenceItCutsThroughMd string
