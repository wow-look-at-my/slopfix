package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

func TestRenamesCaseTypoOfDeclaredName(t *testing.T) {
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
	blocks := AddedBlocks(path, content)
	if len(blocks) == 0 {
		t.Fatal("the comment holds no added block")
	}
	dead := DeadReferents(path, content, blocks)
	if len(dead) != 1 || dead[0] != "NtSCMParse" {
		t.Fatalf("dead = %q, want [NtSCMParse]", dead)
	}
	edits := renameEdits(path, content, blocks, dead)
	if len(edits) != 1 {
		t.Fatalf("renameEdits = %d edits, want 1", len(edits))
	}
	res := treecomments.Apply(path, content, edits, edit.Scope{})
	if len(res.Refused) != 0 || len(res.Applied) != 1 {
		t.Fatalf("gate applied %d, refused %v", len(res.Applied), res.Refused)
	}
	if strings.Contains(res.Text, "NtSCMParse") || !strings.Contains(res.Text, "ntSCMParse collects the SCM_RIGHTS") {
		t.Fatalf("typo not renamed to the declared name: %q", res.Text)
	}
}
