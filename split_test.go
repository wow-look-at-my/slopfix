package slopfix

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
	agents := "# Title\n\nPreamble.\n\n## Big one\n\n" + big + "\n\n### Detail\n\nMore.\n\n## Mid\n\n" + mid + "\n\n## Small\n\n" + small + "\n"
	root := repo(t, map[string]string{"AGENTS.md": agents})

	written, err := Split(root, "AGENTS.md", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/big-one.md"}, written)

	got := readFile(t, root, "AGENTS.md")
	assert.LessOrEqual(t, len([]rune(got)), SplitTarget)
	assert.Contains(t, got, "## Big one\n\n[docs/big-one.md](docs/big-one.md) holds this section.\n\n## Mid")
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
	lines := strings.Split("## Real\n\n```md\n## Not a heading\n```\n\n## Next\n", "\n")
	secs := sections(lines)
	require.Len(t, secs, 2)
	assert.Equal(t, "Real", secs[0].title)
	assert.Equal(t, "Next", secs[1].title)
}

func TestSplitNeverOverwritesAnExistingDoc(t *testing.T) {
	root := repo(t, map[string]string{
		"AGENTS.md":    "## Topic\n\n" + para("x", CharBudget) + "\n",
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
