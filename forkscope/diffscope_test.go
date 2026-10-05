package forkscope

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// DiffLines names the line a selector changed: the work tree against a
// revision, and the index against HEAD with staged. A path the index created
// counts whole.
func TestDiffLinesNamesTheSelectorLines(t *testing.T) {
	work := t.TempDir()
	gitT(t, work, "init", "-q", "-b", "main")
	writeT(t, work, "doc.md", "# Doc\n\nAlpha.\n\nBeta.\n")
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-q", "-m", "base")
	writeT(t, work, "doc.md", "# Doc\n\nAlpha.\n\nChanged.\n")

	own, err := DiffLines(work, "HEAD", false)
	require.NoError(t, err)
	assert.Equal(t, []int{5}, held(own, work, "doc.md", 1, 3, 5), "the work tree diff names the changed line alone")

	writeT(t, work, "new.md", "# New\n")
	gitT(t, work, "add", "-A")
	staged, err := DiffLines(work, "", true)
	require.NoError(t, err)
	assert.Equal(t, []int{5}, held(staged, work, "doc.md", 1, 3, 5))
	assert.True(t, staged.Whole(filepath.Join(work, "new.md")), "a file the index created is the selector's whole")
}

// A selector measures from no commit, so its scope has no base text.
func TestASelectorScopeHasNoBase(t *testing.T) {
	work := t.TempDir()
	gitT(t, work, "init", "-q", "-b", "main")
	writeT(t, work, "doc.md", "# Doc\n")
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-q", "-m", "base")
	writeT(t, work, "doc.md", "# Doc\n\nAdded.\n")

	own, err := DiffLines(work, "HEAD", false)
	require.NoError(t, err)
	_, err = own.Scope(filepath.Join(work, "doc.md")).Base()
	assert.ErrorIs(t, err, errNoBase)
}

// Intersect answers only the lines both scopes name, and a whole scope leaves
// the other.
func TestIntersectKeepsOnlyTheSharedLines(t *testing.T) {
	both := Intersect(OfLines(1, 2, 3), OfLines(2, 3, 4))
	assert.True(t, both.Owns(2))
	assert.True(t, both.Owns(3))
	assert.False(t, both.Owns(1))
	assert.False(t, both.Owns(4))
	assert.True(t, Intersect(Whole(), OfLines(5)).Owns(5))
	assert.True(t, Intersect(OfLines(5), Whole()).Owns(5))
	assert.Nil(t, Intersect(nil, nil))
	assert.True(t, Intersect(nil, OfLines(7)).Owns(7))
}

// IntersectLines answers the same for a tree: a file both wrote whole stays
// whole, and a file one wrote whole takes the other's lines.
func TestIntersectLinesKeepsOnlyTheSharedLines(t *testing.T) {
	a := &Lines{top: "/", whole: set.Of("both"), lines: map[string]set.Set[int]{"x": set.Of(1, 2, 3), "y": set.Of(4)}}
	b := &Lines{top: "/", whole: set.Of("both"), lines: map[string]set.Set[int]{"x": set.Of(2, 3, 4), "y": set.Of(5)}}
	both := IntersectLines(a, b)
	assert.True(t, both.Whole("/both"), "a file both wrote whole stays whole")
	assert.True(t, both.Holds("/x", 2, 2))
	assert.False(t, both.Holds("/x", 1, 1), "a line only one side wrote is not shared")
	assert.False(t, both.Holds("/y", 4, 5), "files that share no line share none")
	assert.Same(t, a, IntersectLines(a, nil))
	assert.Same(t, b, IntersectLines(nil, b))
}
