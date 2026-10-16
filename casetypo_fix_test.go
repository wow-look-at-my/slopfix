package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
)

func TestFixRenamesCaseTypoOfDeclaredName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc readInput() {}\n"), 0o644))
	path := filepath.Join(dir, "os_cosmo_nt_msg.go")
	content := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	require.NotContains(t, fixed.Text, "NtSCMParse")
	require.Contains(t, fixed.Text, "ntSCMParse collects the SCM_RIGHTS")
	require.Empty(t, fixed.Refused)
	require.Empty(t, fixed.Kept)
}
