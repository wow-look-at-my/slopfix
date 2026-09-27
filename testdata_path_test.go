package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// scriptTest is a cmd/go script test. Its lines are commands. The prose rules
// join them, strip the tab from each grep pattern and cut the year from each
// date.
const scriptTest = "# go mod tidy keeps the placeholders too, and needs no go.sum entry for an org\n" +
	"# module. A second tidy leaves go.mod byte-identical.\n" +
	"cd $WORK/m\n" +
	"go mod tidy\n" +
	"grep '^\tgithub.com/wow-look-at-my/alpha v0\\.0\\.0$' go.mod\n" +
	"grep '^\tgithub.com/wow-look-at-my/beta v0\\.0\\.0$' go.mod\n" +
	"! exists go.sum\n" +
	"env GIT_AUTHOR_DATE=2026-02-03T04:05:06Z\n" +
	"exec git commit -am 'beta next'\n"

func TestFixLeavesATestdataFileAlone(t *testing.T) {
	for _, path := range []string{
		"src/cmd/go/testdata/script/org_branch_head.txt",
		`src\cmd\go\testdata\script\org_branch_head.txt`,
		"testdata/notes.md",
	} {
		got := slopfix.Fix(slopfix.Request{Path: path, Content: scriptTest})
		assert.False(t, got.Changed, "%s: a file under testdata is test input, and a rewrite changes what the test checks", path)
		assert.Equal(t, scriptTest, got.Text, path)
		assert.Empty(t, slopfix.CheckContent(path, scriptTest), path)
	}
}

func TestFixStillRepairsTheSameTextOutsideTestdata(t *testing.T) {
	got := slopfix.Fix(slopfix.Request{Path: "docs/script.txt", Content: scriptTest})
	require.True(t, got.Changed, "the control: outside testdata the prose rules own a .txt file, so the exemption above is what spares it")
}
