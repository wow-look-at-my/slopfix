package code

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	ts "github.com/wow-look-at-my/go-tree-sitter"
)

// sameNode reports nodes of one tree at one place with one type.
func sameNode(a, b ts.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Type() == b.Type() && a.IsNamed() == b.IsNamed()
}

// linkedWorktree reports a directory that is a linked worktree of some
// repository: its .git is a file whose gitdir is a directory under
// .git/worktrees. Its files are another checkout's. A submodule's gitdir is
// under .git/modules, and its files are part of this tree.
func linkedWorktree(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return false
	}
	parent := filepath.Dir(filepath.Clean(strings.TrimSpace(gitdir)))
	return filepath.Base(parent) == "worktrees" && filepath.Base(filepath.Dir(parent)) == ".git"
}

// A linked worktree's .git names a gitdir under .git/worktrees. A submodule's
// names one under .git/modules, even inside a linked worktree.
func TestALinkedWorktreeIsAnotherCheckoutAndASubmoduleIsNot(t *testing.T) {
	root := t.TempDir()
	write := func(dir, gitdir string) string {
		path := filepath.Join(root, dir)
		require.NoError(t, os.MkdirAll(path, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644))
		return path
	}
	require.True(t, linkedWorktree(write("wt", "/repo/.git/worktrees/wt")))
	require.False(t, linkedWorktree(write("sub", "../.git/modules/spec")))
	require.False(t, linkedWorktree(write("wtsub", "/repo/.git/worktrees/wt/modules/spec")))
	require.False(t, linkedWorktree(root))
}

// Every file of this repository that a grammar reads: each node's children
// by cursor must be its children by index.
func TestChildrenAnswerAsTheIndexedChildren(t *testing.T) {
	files := 0
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "build", "grammars":
				return filepath.SkipDir
			}
			if path != ".." && linkedWorktree(path) {
				return filepath.SkipDir
			}
			return nil
		}
		language := LanguageFor(path)
		if language == nil {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil || len(src) > 256<<10 {
			return err
		}
		tree := Tree(language, string(src))
		if tree == nil {
			return nil
		}
		files++
		var walk func(n ts.Node)
		walk = func(n ts.Node) {
			all, named := Children(n), NamedChildren(n)
			require.Len(t, all, int(n.ChildCount()), path)
			require.Len(t, named, int(n.NamedChildCount()), path)
			for i, child := range all {
				require.True(t, sameNode(child, n.Child(uint32(i))), "%s: child %d of %s", path, i, n.Type())
			}
			for i, child := range named {
				require.True(t, sameNode(child, n.NamedChild(uint32(i))), "%s: named child %d of %s", path, i, n.Type())
				walk(child)
			}
		}
		walk(tree.RootNode())
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, files, 100)
}
