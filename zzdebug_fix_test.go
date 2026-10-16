package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wow-look-at-my/slopfix"
)

func TestZZDebugFixCaseTypo(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc readInput() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "os_cosmo_nt_msg.go")
	content := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	t.Logf("text=%q", fixed.Text)
	t.Logf("removed=%q", fixed.Removed)
	t.Logf("refused=%q", fixed.Refused)
	for _, h := range fixed.Kept {
		t.Logf("kept %s line %d", h.ID, h.LineNo)
	}
}
