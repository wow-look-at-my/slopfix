package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

func TestRenamesCaseTypoOfDeclaredName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc readInput() {}\n"), 0o644))
	path := filepath.Join(dir, "os_cosmo_nt_msg.go")
	content := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"
	blocks := AddedBlocks(path, content)
	require.NotEmpty(t, blocks, "the comment holds an added block")
	dead := DeadReferents(path, content, blocks)
	require.Equal(t, []string{"NtSCMParse"}, dead)
	edits := renameEdits(path, content, blocks, dead)
	require.Len(t, edits, 1)
	res := treecomments.Apply(path, content, edits, edit.Scope{})
	require.Empty(t, res.Refused)
	require.Len(t, res.Applied, 1)
	require.NotContains(t, res.Text, "NtSCMParse")
	require.Contains(t, res.Text, "ntSCMParse collects the SCM_RIGHTS")
}
