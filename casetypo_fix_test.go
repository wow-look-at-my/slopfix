package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix"
)

func TestFixRenamesCaseTypoOfDeclaredName(t *testing.T) {
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
	if strings.Contains(fixed.Text, "NtSCMParse") || !strings.Contains(fixed.Text, "ntSCMParse collects the SCM_RIGHTS") {
		t.Fatalf("typo not renamed to the declared name: %q", fixed.Text)
	}
	if len(fixed.Refused) != 0 || len(fixed.Kept) != 0 {
		t.Fatalf("fix left refused %v kept %v", fixed.Refused, fixed.Kept)
	}
}
