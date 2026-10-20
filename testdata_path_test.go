package slopfix_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

//go:embed testdata/script-test.sh
var scriptTest string

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
